import path from "path";
import fs from "fs";
import os from "os";
import { addStatus, BuildStatus, runNewProcessWithResult, unzipArchive, } from "./utils.js";

console.log("Starting build from s3 flow");
console.log(process.argv)

const token = process.argv[2];

deployFromArchive({
  token
});

async function deployFromArchive(params) {
  const tmpDir = "/tmp/projectCode"
  let statusArray = []
  await addStatus(BuildStatus.PENDING, "Starting build from s3 flow", statusArray);
  console.log("Unzipping code")
  await unzipArchive("/tmp/projectCode.zip", tmpDir);

  const token = params.token;
  if (!token) {
    await addStatus(BuildStatus.FAILED, "Invalid token", statusArray);
    throw Error("Invalid request");
  }

  console.log("Deploying code");

  // deploy the code
  // npm i
  await addStatus(BuildStatus.INSTALLING_DEPS, "Installing dependencies", statusArray);
  console.log("Installing dependencies");
  const installResult = await runNewProcessWithResult(
    `npm i`,
    tmpDir
  ).catch(async e => {
    await addStatus(BuildStatus.FAILED, `Failed to install dependencies ${installResult.stderr}`, statusArray);
    throw Error("Failed to install dependencies", e);
  });

  console.log("Installed dependencies");
  await addStatus(BuildStatus.AUTHENTICATING, "Authenticating with genezio", statusArray);
  const loginResult = await runNewProcessWithResult(
    `genezio login ${token}`,
    tmpDir
  )
  if (!loginResult || loginResult.code !== 0) {
    console.log(loginResult.stdout)
    console.log(loginResult.stderr)
    await addStatus(BuildStatus.FAILED, `Failed to login ${loginResult.stderr}`, statusArray);
    throw Error(`Failed to login ${loginResult.stdout} ${loginResult.stderr}`);
  }
  console.log("Logged in");

  console.log("Deploying...");
  await addStatus(BuildStatus.DEPLOYING, "Deploying project", statusArray);
  const deployResult = await runNewProcessWithResult(
    `CI=true genezio deploy`,
    tmpDir
  )

  if (!deployResult || deployResult.code !== 0) {
    console.log(deployResult.stdout)
    console.log(deployResult.stderr)
    await addStatus(BuildStatus.FAILED, `Failed to deploy ${deployResult.stderr}`, statusArray);
    throw Error(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }

  await addStatus(BuildStatus.SUCCESS, "Workflow completed successfully", statusArray);
  console.log("Deployed");

  console.log("DONE Deploying, sending response");
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
