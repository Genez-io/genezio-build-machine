import path from "path";
import fs from "fs";
import os from "os";
import { runNewProcessWithResult, unzipArchive, prepareGithubRepository, zipDirectory, uploadContentToS3 } from "./utils.js";
import axios from "axios";

console.log("Starting empty project flow");
console.log(process.argv)

try {
  const token = process.argv[2];
  const githubRepository = process.argv[3];
  const projectName = process.argv[4];
  const region = process.argv[5];
  const basePath = process.argv[6];

  deployEmpty({
    token, githubRepository, projectName, region, basePath
  });
} catch (error) {
  console.error("Failed to deploy", error);
}

async function deployEmpty(params) {
  console.log(params)
  const { token, githubRepository, projectName, region, basePath } = params;
  if (!token || !githubRepository) {
    throw Error("Invalid request");
  }

  const tmpDir = await prepareGithubRepository(githubRepository, projectName, region, basePath).catch(e => {
    return e;
  });

  if (tmpDir instanceof Error) {
    throw Error(tmpDir.message);
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
        stack: [],
      }
    })
  } catch (e) {
    console.error("Failed to deploy project");
    console.log(e)
    throw Error("Failed to deploy empty project");
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
    throw Error("Failed to create project code url", e);
  });
  if (!response || !response.data.presignedURL) {
    throw Error("Failed to create project code url");
  }
  const url = response.data.presignedURL;
  if (!url) {
    throw Error("Failed to create project code url");
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
    throw Error("Failed to upload code to S3");
  }

  console.log("Deployed successfully");
  process.exit(0);
}
