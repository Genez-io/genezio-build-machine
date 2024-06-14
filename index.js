import express from "express";
import path from "path";
import fs from "fs";
import { exec } from "child_process";
import os from "os";
import { parse, stringify } from "yaml-transmute";

const app = express();

// allow all cors
app.use((req, res, next) => {
  res.header("Access-Control-Allow-Origin", "*");
  res.header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE");
  res.header("Access-Control-Allow-Headers", "*");
  next();
});

// change timeout to 10 minutes
app.use((req, res, next) => {
  res.setTimeout(600000, () => {
    console.log("Request has timed out.");
    res.send(408);
  });
  next();
});

app.use(express.json());

app.get("/healthcheck", (req, res) => {
  res.status(200).send("OK");
});

app.post("/deploy", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, code } = body;

  if (!token || !code) {
    return res.status(400).send("Invalid request");
  }

  if (!code["genezio.yaml"]) {
    return res.status(400).send("genezio.yaml is required");
  }

  // create a temporary directory
  const tmpDir = await createTemporaryFolder();

  // get all keys of code
  await Promise.all([
    ...Object.keys(code).map(async key => {
      const codeFile = code[key];

      // get fileName from key
      const fileName = key.split("/").pop();
      const finalPath = path.join(tmpDir, ...key.split("/").slice(0, -1));
      await writeToFile(finalPath, fileName, codeFile, true);
    })
  ]);

  // deploy the code
  // npm i
  console.log("Installing dependencies");
  const installResult = await runNewProcessWithResult(
    `npm i`,
    tmpDir
  ).catch(e => {
    console.error("Failed to install dependencies", e);
    return null;
  });
  if (!installResult) {
    return res
      .status(500)
      .send(
        `Failed to install dependencies ${installResult.stdout} ${installResult.stderr}`
      );
  }
  console.log("Installed dependencies");
  console.log("Deploying...");
  const deployResult = await runNewProcessWithResult(
    `GENEZIO_TOKEN=${token} genezio deploy`,
    tmpDir
  ).catch(e => {
    console.error("Failed to deploy", e);
    return null;
  });

  if (!deployResult || deployResult.code !== 0) {
    return res
      .status(500)
      .send(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }
  console.log("Deployed");

  cleanUp(tmpDir);

  return res.status(200).send("Deployed successfully");
});

app.post("/github-deploy", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, githubRepository, projectName } = body;

  if (!token || !githubRepository) {
    return res.status(400).send("Invalid request");
  }

  // create a temporary directory
  const tmpDir = await createTemporaryFolder();

  // check if the repository and check if 200
  const resCheckRepo = await fetch(githubRepository).catch(e => {
    console.error("Failed to fetch repository", e);
    return null;
  });
  if (!resCheckRepo || resCheckRepo.status !== 200) {
    return res
      .status(500)
      .send("Failed to fetch the repository. It may not exist or is private");
  }

  // clone the repository
  console.log("Cloning repository");
  const cloneResult = await runNewProcessWithResult(
    `git clone ${githubRepository} .`,
    tmpDir
  ).catch(e => {
    console.error("Failed to clone repository", e);
    return null;
  });

  if (!cloneResult || cloneResult.code !== 0) {
    return res
      .status(500)
      .send(
        `Failed to clone repository ${cloneResult.stdout} ${cloneResult.stderr}`
      );
  }

  if (!fs.existsSync(path.join(tmpDir, "genezio.yaml"))) {
    return res
      .status(500)
      .send("genezio.yaml is required and it was not found in the repository");
  }

  const resDeps = await checkAndInstallDeps(tmpDir).catch(e => {
    return null;
  });

  if (!resDeps) {
    return res.status(500).send("Failed to install dependencies");
  }

  if (projectName) {
    const yamlPath = path.join(tmpDir, "genezio.yaml");
    const yamlContent = fs.readFileSync(yamlPath, "utf-8");
    const [yaml, ctx] = parse(yamlContent);

    yaml.name = projectName;

    const newYamlContent = stringify(yaml, ctx);
    fs.writeFileSync(yamlPath, newYamlContent);
  }

  // deploy the code
  console.log("Deploying...");
  const deployResult = await runNewProcessWithResult(
    `GENEZIO_TOKEN=${token} genezio deploy`,
    tmpDir
  ).catch(e => {
    console.error("Failed to deploy", e);
    return null;
  });

  if (!deployResult || deployResult.code !== 0) {
    return res
      .status(500)
      .send(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }
  console.log("Deployed");

  cleanUp(tmpDir);

  return res.status(200).send("Deployed successfully");
});

export async function createTemporaryFolder() {
  return new Promise((resolve, reject) => {
    // eslint-disable-next-line no-undef
    const folderName = `genezio-${process.pid}`;

    if (!fs.existsSync(path.join(os.tmpdir(), folderName))) {
      fs.mkdirSync(path.join(os.tmpdir(), folderName));
    }

    const name = Math.random().toString(36).substring(2, 8);

    const tempFolder = path.join(os.tmpdir(), folderName, name);
    if (fs.existsSync(tempFolder)) {
      fs.rmSync(tempFolder, { recursive: true });
    }

    fs.mkdir(tempFolder, error => {
      if (error) {
        reject(error);
      }

      resolve(tempFolder);
    });
  });
}

export function writeToFile(
  folderPath,
  filename,
  content,
  createPathIfNeeded = false
) {
  return new Promise((resolve, reject) => {
    const fullPath = path.join(folderPath, filename);

    if (!fs.existsSync(path.dirname(fullPath)) && createPathIfNeeded) {
      fs.mkdirSync(path.dirname(fullPath), { recursive: true });
    }

    // create the file if it doesn't exist
    fs.writeFile(fullPath, content, function(error) {
      if (error) {
        reject(error);
        return;
      }

      resolve();
    });
  });
}

app.listen(8080, () => {
  console.log("Server running on port 8080");
});

export function runNewProcessWithResult(command, cwd) {
  return new Promise(function(resolve) {
    exec(command, { cwd }, (err, stdout, stderr) => {
      console.log("stdout", stdout);
      console.log("stderr", stderr);
      if (err) {
        resolve({ code: err.code, stdout, stderr });
      } else {
        resolve({ code: 0, stdout, stderr });
      }
    });
  });
}

export async function checkAndInstallDeps(path) {
  // Check if next.config.js exists
  if (
    fs.existsSync(`${path}/next.config.js`) ||
    fs.existsSync(`${path}/next.config.mjs`)
  ) {
    console.log("Installing dependencies for next");
    const installResult = await runNewProcessWithResult(
      `npm i`,
      path
    ).catch(e => {
      console.error("Failed to install dependencies", e);
      return null;
    });
    if (!installResult) {
      throw `Failed to install dependencies ${installResult.stdout} ${installResult.stderr}`;
    }
  }

  console.log("DONE Installing dependencies");

  return true;
}

async function cleanUp(path) {
  fs.rmSync(path, { recursive: true });
}
