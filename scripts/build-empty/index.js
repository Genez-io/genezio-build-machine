import path from "path";
import fs from "fs";
import { mkdirSync } from "fs";
import { prepareGithubRepository, zipDirectory, uploadContentToS3 } from "./utils.js";
import axios from "axios";

console.log("Starting empty project flow");
console.log(process.argv)

const token = process.argv[2];
const githubRepository = process.argv[3];
const projectName = process.argv[4];
const region = process.argv[5];
const stack = process.argv[6];
const basePath = process.argv[7];

deployEmpty({
    token, githubRepository, projectName, region, stack, basePath
});

async function deployEmpty(params) {
  console.log(params)
  const { token, githubRepository, projectName, region, stack, basePath } = params;
  if (!token || !githubRepository) {
    throw Error("Invalid request");
  }

  let stackParsed = [];
  if (stack !== "[]") {
    stackParsed = stack.split(",")
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
        stack: stackParsed,
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
  const zipdir = path.join("tmp", "projectCode");
  const zipdirfile = path.join(zipdir, "projectCode.zip");
  mkdirSync(zipdir, { recursive: true });
  await zipDirectory(tmpDir, zipdirfile, [
    "projectCode.zip",
    "**/projectCode.zip",
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
    await uploadContentToS3(url, zipdirfile)
  } catch (e) {
    console.error("Failed to upload code to S3", e);
    throw Error("Failed to upload code to S3");
  }
 
  cleanUp(tmpDir);
  cleanUp("tmp");

  console.log("Deployed successfully");
  process.exit(0);
}

async function cleanUp(path) {
  fs.rmSync(path, { recursive: true });
}
