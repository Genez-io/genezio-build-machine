import "dotenv/config";
import express from "express";
import path from "path";
import fs from "fs";
import { exec } from "child_process";
import os from "os";
import { parse, stringify } from "yaml-transmute";
import axios from "axios";
import { uploadContentToS3, zipDirectory } from "./utils.js";

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

app.use(express.json({ limit: "250mb" }));

app.get("/healthcheck", (req, res) => {
  res.status(200).send("OK");
});

app.post("/deploy", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, code, basePath } = body;

  if (!token || !code) {
    return res.status(400).send("Invalid request");
  }

  if (!code["genezio.yaml"]) {
    return res.status(400).send("genezio.yaml is required");
  }

  console.log("Deploying code");

  // create a temporary directory
  let tmpDir = await createTemporaryFolder();
  if (basePath) {
    tmpDir = path.join(tmpDir, basePath);
  }

  console.log("Created temporary directory", tmpDir);

  // get all keys of code
  const resAll = await Promise.all([
    ...Object.keys(code).map(async key => {
      const codeFile = code[key];

      // get fileName from key
      const fileName = key.split("/").pop();
      const finalPath = path.join(tmpDir, ...key.split("/").slice(0, -1));
      await writeToFile(finalPath, fileName, codeFile, true);
      return true;
    })
  ]).catch(e => {
    console.error("Failed to write files", e);
    return null;
  });

  if (!resAll) {
    await cleanUp(tmpDir);
    return res.status(500).send("Failed to write files");
  }

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
    await cleanUp(tmpDir);
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
    await cleanUp(tmpDir);
    return res
      .status(500)
      .send(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }
  console.log("Deployed");

  await cleanUp(tmpDir).catch(e => {
    console.error("Failed to clean up", e);
  });

  console.log("DONE Deploying, sending response");
  return res.status(200).send("Deployed successfully");
});

app.post("/github-deploy", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, githubRepository, projectName, region, basePath } = body;

  if (!token || !githubRepository) {
    return res.status(400).send("Invalid request");
  }

  const tmpDir = await prepareGithubRepository(token, githubRepository, projectName, region, basePath).catch(e => {
    return e;
  });

  if (tmpDir instanceof Error) {
    return res.status(500).send(tmpDir.message);
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
    await cleanUp(tmpDir).catch(e => {
      console.error("Failed to clean up", e);
    });
    return res
      .status(500)
      .send(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }
  console.log("Deployed");

  await cleanUp(tmpDir);

  console.log("DONE Deploying, sending response");
  return res.status(200).send("Deployed successfully");
});

app.post("/deploy-empty-project", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, githubRepository, projectName, region, basePath, stack } = body;

  if (!token || !githubRepository) {
    return res.status(400).send("Invalid request");
  }

  const tmpDir = await prepareGithubRepository(token, githubRepository, projectName, region, basePath).catch(e => {
    return e;
  });

  if (tmpDir instanceof Error) {
    return res.status(500).send(tmpDir.message);
  }

  // deploy an empty project
  try {
    await axios({
      method: "PUT",
      // eslint-disable-next-line no-undef
      url: process.env.GENEZIO_API_BASE_URL + "/core/deployment",
      headers: {
        Authorization: `Bearer ${token}`,
        "Accept-Version": "genezio-cli/2.0.3"
      },
      data: {
        projectName,
        region,
        cloudProvider: "genezio-cloud",
        stage: "prod",
        stack,
      }
    })
  } catch (e) {
    console.error("Failed to deploy project", e);
    return res.status(500).send("Failed to deploy empty project");
  }
  // get s3 presigned url
  const response = await axios({
    method: "POST",
    // eslint-disable-next-line no-undef
    url: `${process.env.GENEZIO_API_BASE_URL}/core/create-project-code-url`,
    headers: {
      Authorization: `Bearer ${token}`,
      "Accept-Version": "genezio-cli/2.0.3"
    },
    data: {
      projectName,
      region,
      stage: "prod"
    }
  }).catch(e => {
    console.error("Failed to create project code url", e);
    return null;
  });
  if (!response || !response.data.presignedURL) {
    return res.status(500).send("Failed to create project code url");
  }
  const url = response.data.presignedURL;
  if (!url) {
    return res.status(500).send("Failed to create project code url");
  }
  //upload code to S3
  await zipDirectory(tmpDir, path.join(tmpDir, "projectCode.zip"), [
    "**/node_modules/*",
    "./node_modules/*",
    "node_modules/*",
    "**/node_modules",
    "./node_modules",
    "node_modules",
    "node_modules/**",
    "**/node_modules/**",
    // ignore all .git files
    "**/.git/*",
    "./.git/*",
    ".git/*",
    "**/.git",
    "./.git",
    ".git",
    ".git/**",
    "**/.git/**",
    // ignore all .next files
    "**/.next/*",
    "./.next/*",
    ".next/*",
    "**/.next",
    "./.next",
    ".next",
    ".next/**",
    "**/.next/**",
    // ignore all .open-next files
    "**/.open-next/*",
    "./.open-next/*",
    ".open-next/*",
    "**/.open-next",
    "./.open-next",
    ".open-next",
    ".open-next/**",
    "**/.open-next/**",
    // ignore all .vercel files
    "**/.vercel/*",
    "./.vercel/*",
    ".vercel/*",
    "**/.vercel",
    "./.vercel",
    ".vercel",
    ".vercel/**",
    "**/.vercel/**",
    // ignore all .turbo files
    "**/.turbo/*",
    "./.turbo/*",
    ".turbo/*",
    "**/.turbo",
    "./.turbo",
    ".turbo",
    ".turbo/**",
    "**/.turbo/**",
    // ignore all .sst files
    "**/.sst/*",
    "./.sst/*",
    ".sst/*",
    "**/.sst",
    "./.sst",
    ".sst",
    ".sst/**",
    "**/.sst/**",
  ]);
  try {
    await uploadContentToS3(url, path.join(tmpDir, "projectCode.zip"))
  } catch (e) {
    console.error("Failed to upload code to S3", e);
    return res.status(500).send("Failed to upload code to S3");
  }

  await cleanUp(tmpDir);
  res.status(200).send("Deployed successfully");
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

async function prepareGithubRepository(token, githubRepository, projectName, region, basePath) {
  console.log("Deploying code from github");
  console.log("Repository", githubRepository);
  console.log("Project Name", projectName);
  console.log("Region", region);

  // create a temporary directory
  let tmpDir = await createTemporaryFolder();
  console.log("Created temporary directory", tmpDir);

  if (basePath) {
    tmpDir = path.join(tmpDir, basePath);
  }

  // check if the repository and check if 200
  const resCheckRepo = await fetch(githubRepository).catch(e => {
    console.error("Failed to fetch repository", e);
    return null;
  });
  if (!resCheckRepo || resCheckRepo.status !== 200) {
    throw new Error("Failed to fetch the repository. It may not exist or is private");
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
    throw new Error(`Failed to clone repository ${cloneResult.stdout} ${cloneResult.stderr}`)
  }

  if (!fs.existsSync(path.join(tmpDir, "genezio.yaml"))) {
    throw new Error("genezio.yaml is required and it was not found in the repository");
  }

  const resDeps = await checkAndInstallDeps(tmpDir).catch(e => {
    return null;
  });

  if (!resDeps) {
    await cleanUp(tmpDir).catch(e => {
      console.error("Failed to clean up", e);
    });
    throw new Error("Failed to install dependencies");
  }

  try {
    if (projectName && region) {
      const yamlPath = path.join(tmpDir, "genezio.yaml");
      const yamlContent = fs.readFileSync(yamlPath, "utf-8");
      const [yaml, ctx] = parse(yamlContent);

      yaml.name = projectName;
      yaml.region = region;

      const newYamlContent = stringify(yaml, ctx);
      fs.writeFileSync(yamlPath, newYamlContent);
    }
  } catch (e) {
    console.error("Failed to update genezio.yaml", e);
    throw new Error("Failed to update genezio.yaml");
  }

  return tmpDir;
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
