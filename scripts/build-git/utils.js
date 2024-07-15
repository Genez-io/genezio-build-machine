import fs from "fs";
import archiver from "archiver";
import https from "https";
import { spawn } from "child_process";
import path from "path";
import os from "os"
import { parse, stringify } from "yaml-transmute";
export async function zipDirectory(
  sourceDir,
  outPath,
  exclusion,
) {
  const archive = archiver("zip", { zlib: { level: 9 } });
  const stream = fs.createWriteStream(outPath);

  if (!exclusion) {
    exclusion = [];
  }

  return new Promise((resolve, reject) => {
    archive
      .glob("**/*", {
        cwd: sourceDir,
        dot: true,
        skip: exclusion,
      })
      .on("error", (err) => reject(err))
      .pipe(stream);

    stream.on("close", () => resolve());
    archive.finalize();
  });
}

export async function unzipArchive(
  sourcePath,
  outDir,
) {
  try {
    await runNewProcessWithResult(`unzip -o ${sourcePath} -d ${outDir}`, path.dirname(sourcePath));
  } catch (error) {
    console.error("Failed to unzip archive", error);
    throw error;
  }
}
export async function uploadContentToS3(
  presignedURL,
  archivePath,
) {
  if (!presignedURL) {
    throw new Error("Missing presigned URL");
  }

  if (!archivePath) {
    throw new Error("Missing required parameters");
  }

  // Check if user is authenticated
  const url = new URL(presignedURL);

  const headers = {
    "Content-Type": "application/octet-stream",
    "Content-Length": fs.statSync(archivePath).size,
  };

  const options = {
    hostname: url.hostname,
    path: url.href,
    port: 443,
    method: "PUT",
    headers: headers,
  };

  return await new Promise((resolve, reject) => {
    const req = https.request(options, (res) => {
      // If we don't consume the data, the "end" event will not fire
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      res.on("data", () => { });

      res.on("end", () => {
        resolve();
      });
    });

    req.on("error", (error) => {
      reject(error);
    });

    const fileStream = fs.createReadStream(archivePath);

    fileStream
      .on("data", () => { })
      .pipe(req);
  });
}

export function runNewProcessWithResult(command, args, cwd, env) {
  return new Promise(function (resolve) {
    const child = spawn(command, args, { cwd, env: { ...process.env, ...env } });

    let stdout = '';
    let stderr = '';

    child.stdout.on('data', (data) => {
      stdout += data.toString();
      console.log("stdout", data.toString());
    });

    child.stderr.on('data', (data) => {
      stderr += data.toString();
      console.log("stderr", data.toString());
    });

    child.on('close', (code, signal) => {
      if (`${signal}` === 'SIGSEGV') {
        code = 0;
      }
      resolve({ code, stdout, stderr });
    });

    child.on('error', (err) => {
      resolve({ code: err.code, stdout, stderr });
    });

    child.stdin.end()
  });
}

export async function prepareGithubRepository(githubRepository, projectName, region, basePath) {
  console.log("Deploying code from github");
  console.log("Repository", githubRepository);
  console.log("Project Name", projectName);
  console.log("Region", region);

  // create a temporary directory
  let tmpDir = await createTemporaryFolder();
  console.log("Created temporary directory", tmpDir);

  // check if the repository and check if 200
  const resCheckRepo = await fetch(githubRepository).catch(e => {
    console.error("Failed to fetch repository", e);
    return null;
  });
  if (!resCheckRepo || resCheckRepo.status !== 200) {
    throw new Error("Failed to fetch the repository. It may not exist or is private");
  }

  // clone the repository
  console.log("Cloning repository", githubRepository, tmpDir);
  const cloneResult = await runNewProcessWithResult(
    `git`, ['clone', githubRepository, '.'],
    tmpDir
  ).catch(e => {
    console.log(e)
    throw Error("Failed to clone repository", e);

  });

  if (!cloneResult || cloneResult.code !== 0) {
    console.log(cloneResult)
    throw new Error(`Failed to clone repository ${cloneResult.stdout} ${cloneResult.stderr}`)
  }

  if (basePath && basePath.length > 0) {
    tmpDir = path.join(tmpDir, basePath);
  }
  if (!fs.existsSync(path.join(tmpDir, "genezio.yaml"))) {
    // create file
    const content = `name: ${projectName}\nregion: ${region}\nyamlVersion: 2\n`;

    await writeToFile(tmpDir, "genezio.yaml", content, true).catch(e => {
      console.error("Failed to create genezio.yaml", e);
      throw new Error("Failed to create genezio.yaml");
    });
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

      const oldYamlName = yaml.name;
      yaml.name = projectName;
      yaml.region = region;

      const newYamlContent = stringify(yaml, ctx);
      fs.writeFileSync(yamlPath, newYamlContent);
      // replace old project name in the entire project
      await recursiveReplace(tmpDir, [
        [`@genezio-sdk/${oldYamlName}`, `@genezio-sdk/${projectName}`],
      ]);
    }
  } catch (e) {
    console.error("Failed to update genezio.yaml", e);
    throw new Error("Failed to update genezio.yaml");
  }

  return tmpDir;
}

async function recursiveReplace(
  rootPath,
  replacements
) {
  const fromStats = fs.statSync(rootPath);
  if (fromStats.isDirectory()) {
    // @ts-expect-error TypeScript does not infer the function type correctly
    const files = fs.readdirSync(rootPath);
    for (const file of files) {
      recursiveReplace(path.join(rootPath, file), replacements);
    }
  } else {
    const fileContent = fs.readFileSync(rootPath, "utf8");

    const newFileContent = replacements.reduce(
      (acc, [placeholder, value]) => acc.replaceAll(placeholder, value),
      fileContent,
    );

    if (newFileContent !== fileContent) {
      fs.writeFileSync(rootPath, newFileContent);
    }
  }
}

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
    fs.writeFile(fullPath, content, function (error) {
      if (error) {
        reject(error);
        return;
      }

      resolve();
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
      `npm`, [`i`],
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
