import fs from "fs";
import archiver from "archiver";
import https from "https";
import { exec } from "child_process";
import path from "path";

export const BuildStatus = {
  PENDING: "PENDING",
  AUTHENTICATING: "AUTHENTICATING",
  PULLING_CODE: "PULLING_CODE",
  INSTALLING_DEPS: "INSTALLING_DEPS",
  BUILDING: "BUILDING",
  DEPLOYING: "DEPLOYING",
  DEPLOYING_BACKEND: "DEPLOYING_BACKEND",
  DEPLOYING_FRONTEND: "DEPLOYING_FRONTEND",
  SUCCESS: "SUCCESS",
  FAILED: "FAILED"
};

export async function addStatus(status, message, statusArray) {
  const statusFile = path.join("/tmp", "status.json");
  statusArray.push({ status, message, time: new Date().toISOString() });
  fs.writeFile(statusFile, JSON.stringify(statusArray), { mode: 0o777 }, (err) => {
    if (err) {
      console.error("Failed to write status file", err);
    }
  })
  if (status === "FAILED") {
    // Sleep 5 seconds to allow the status file to be read
    // before the process exits
    // This is a workaround for the status file not being read in time, will be removed
    // once we properly setup state storage in a persistent database
    console.log("Sleeping for 5 seconds, waiting status read");
    await new Promise(r => setTimeout(r, 5000));
  }
}

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

function delay(time) {
  return new Promise(resolve => setTimeout(resolve, time));
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

export function runNewProcessWithResult(command, cwd) {
  return new Promise(function (resolve) {
    exec(command, { cwd, }, (err, stdout, stderr) => {
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