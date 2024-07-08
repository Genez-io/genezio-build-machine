import path from "path";
import fs from "fs";
import os from "os";
import { runNewProcessWithResult, unzipArchive, prepareGithubRepository } from "./utils.js";

console.log("Starting build from git flow");
console.log(process.argv)

const token = process.argv[2];
const githubRepository = process.argv[3];
const projectName = process.argv[4];
const region = process.argv[5];
const basePath = process.argv[6];

console.log(process.argv)
deployFromGit({
    token, githubRepository, projectName, region, basePath
});

async function deployFromGit(params) {
  console.log(params)
  const { token, githubRepository, projectName, region, basePath } = params;
  if (!token || !githubRepository) {
    throw Error("Invalid request");
  }

  const loginResult = await runNewProcessWithResult(
    `genezio login ${token}`,
    tmpDir
  ).catch(e => {
    throw Error("Failed to deploy", e);
  });
  if (!loginResult || loginResult.code !== 0) {
    console.log(loginResult.stdout)
    console.log(loginResult.stderr)
    throw Error(`Failed to login ${loginResult.stdout} ${loginResult.stderr}`);
  }
  console.log("Logged in");

  const tmpDir = await prepareGithubRepository(githubRepository, projectName, region, basePath)

  if (tmpDir instanceof Error) {
    console.log(tmpDir)
    throw Error(tmpDir);
  }

  // deploy the code
  console.log("Deploying...");

  const deployResult = await runNewProcessWithResult(
    `genezio deploy`,
    tmpDir
  ).catch(e => {
    throw Error("Failed to deploy", e);
  });

  if (!deployResult || deployResult.code !== 0) {
    throw Error(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }
  console.log("Deployed");

  console.log("DONE Deploying, sending response");
  process.exit(0);
}

async function cleanUp(path) {
  fs.rmSync(path, { recursive: true });
}
