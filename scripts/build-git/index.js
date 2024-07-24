import fs from "fs";
import { runNewProcessWithResult, prepareGithubRepository, addStatus, BuildStatus } from "./utils.js";


console.log("Starting build from git flow");
console.log(process.argv)

const token = process.argv[2];
const githubRepository = process.argv[3];
const projectName = process.argv[4];
const region = process.argv[5];
const basePath = process.argv[6];
let stack = null;

try {
    stack = JSON.parse(process.argv[7]);
} catch (e) {
    console.log("Stack does not exist")
}
const isNewProject = process.argv[8] === "true";

console.log(process.argv)
deployFromGit({
  token, githubRepository, projectName, region, basePath, isNewProject, stack
});

async function deployFromGit(params) {
  let statusArray = []
  await addStatus(BuildStatus.PENDING, "Starting build from git flow", statusArray);
  console.log(params)
  const { token, githubRepository, projectName, region, basePath, isNewProject, stack } = params;
  if (!token || !githubRepository) {
    throw Error("Invalid request");
  }
  await addStatus(BuildStatus.AUTHENTICATING, "Authenticating with genezio", statusArray);
  const loginResult = await runNewProcessWithResult(
    `genezio`, ['login', token],
  );
  if (!loginResult || loginResult.code !== 0) {
    console.log(loginResult.stdout)
    console.log(loginResult.stderr)
    await addStatus(BuildStatus.FAILED, loginResult.stderr, statusArray);
    throw Error(`Failed to login ${loginResult.stdout} ${loginResult.stderr}`);
  }
  console.log("Logged in");

  await addStatus(BuildStatus.PULLING_CODE, "Pulling code from github", statusArray);

  let deployDir = ""
  try {
    const tmpDir = await prepareGithubRepository(token, githubRepository, projectName, region, basePath, isNewProject, stack, statusArray)
    if (tmpDir instanceof Error) {
      await addStatus(BuildStatus.FAILED, `Failed to clone github repository ${githubRepository}`, statusArray);
      console.log(tmpDir)
      throw Error(tmpDir);
    }
    deployDir = tmpDir
  } catch (error) {
    await addStatus(BuildStatus.FAILED, `${error.toString()}`, statusArray);
    return
  }

  // deploy the code
  console.log("Deploying...");
  await addStatus(BuildStatus.DEPLOYING, "Deploying project", statusArray);
  const deployResult = await runNewProcessWithResult(
    `genezio`,
    [`deploy`],
    deployDir,
    {
      "CI": true
    }
  ).catch(async e => {
    await addStatus(BuildStatus.FAILED, `${e.toString()}`, statusArray);
    throw Error("Failed to deploy", e);
  });

  console.log(deployResult);
  if (!deployResult || deployResult.code !== 0) {
    await addStatus(BuildStatus.FAILED, deployResult.stderr, statusArray);
    throw Error(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }

  await addStatus(BuildStatus.SUCCESS, "Workflow completed successfully", statusArray);
  console.log("Deployed");

  console.log("DONE Deploying, sending response");
  process.exit(0);
}

async function cleanUp(path) {
  fs.rmSync(path, { recursive: true });
}
