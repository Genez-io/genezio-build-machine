import {
  runNewProcessWithResult,
  cloneRepository,
  addStatus,
  BuildStatus,
  writeConfigurationFileIfNeeded,
  createNewProject,
  replaceGenezioImports,
  StatusEntry,
  checkAndInstallDeps,
  writeEnvsToFile,
  createTemporaryFolder,
} from "./utils.js";

type InputParams = {
  token: string;
  githubRepository: string;
  projectName: string;
  region: string;
  basePath: string;
  isNewProject: boolean;
  stack: string[] | null;
  stage: string;
  envVars: { [key: string]: string };
};

export async function runGitFlow() {
  console.log("Starting build from git flow");

  const input = parseArguments();
  const statusArray: StatusEntry[] = [];
  try {
    await deployFromGit(input);
  } catch (e: any) {
    await addStatus(BuildStatus.FAILED, e.toString(), statusArray);
  }
}

function parseArguments(): InputParams {
  const paramsB64 = process.argv[3];
  const params: InputParams = JSON.parse(
    Buffer.from(paramsB64, "base64").toString("utf-8")
  );
  console.log(params);

  const token = params.token;
  const githubRepository = params.githubRepository;
  const projectName = params.projectName;
  const region = params.region;
  const basePath = params.basePath;
  let stack = params.stack;
  if (!stack) {
    stack = [];
  }

  const envVars = params.envVars;
  const isNewProject = params.isNewProject;
  const stage = params.stage;

  return {
    token,
    githubRepository,
    projectName,
    region,
    basePath,
    isNewProject,
    stack,
    stage,
    envVars,
  };
}

async function deployFromGit(
  params: InputParams,
  statusArray: StatusEntry[] = []
) {
  await addStatus(
    BuildStatus.SCHEDULED,
    "Starting build from git flow",
    statusArray
  );

  let {
    token,
    githubRepository,
    projectName,
    region,
    basePath,
    isNewProject,
    stack,
    stage,
    envVars,
  } = params;
  if (!token || !githubRepository) {
    throw Error("Invalid request");
  }
  await addStatus(
    BuildStatus.AUTHENTICATING,
    "Authenticating with genezio",
    statusArray
  );
  const loginResult = await runNewProcessWithResult(`genezio`, [
    "login",
    token,
  ]);
  if (!loginResult || loginResult.code !== 0) {
    console.log(loginResult.stdout);
    console.log(loginResult.stderr);
    throw Error(`Authenticating failed: ${loginResult.stderr}`);
  }
  console.log("Logged in");

  await addStatus(
    BuildStatus.PULLING_CODE,
    "Pulling code from github",
    statusArray
  );

  const folder = await cloneRepository(githubRepository, basePath, stage);
  await writeConfigurationFileIfNeeded(folder, projectName, region);

  if (stage === "main" || stage === "master" || stage === "") {
    stage = "prod";
  }

  await addStatus(
    BuildStatus.CREATING_PROJECT,
    "Creating project",
    statusArray
  );
  await replaceGenezioImports(projectName, region, folder);

  if (isNewProject) {
    await addStatus(
      BuildStatus.CREATING_EMPTY_PROJECT,
      "Creating new project",
      statusArray
    );
    await createNewProject(token, projectName, region, stack, folder, stage);
  }

  console.log("Installing dependencies if needed...");
  await checkAndInstallDeps(folder, statusArray);

  // deploy the code
  console.log("Deploying...");
  await addStatus(BuildStatus.DEPLOYING, "Deploying project", statusArray);

  let deployArgs = [`deploy`, `--stage`, `${stage}`];
  if (Object.keys(envVars).length > 0) {
    deployArgs.push(`--env`);
    const envFileName = await writeEnvsToFile(envVars, folder);
    deployArgs.push(envFileName);
  }

  const deployResult = await runNewProcessWithResult(
    `genezio`,
    deployArgs,
    folder,
    {
      CI: true,
    }
  ).catch(async (e) => {
    throw Error(`Failed to deploy ${e}`);
  });

  console.log(deployResult);
  if (!deployResult || deployResult.code !== 0) {
    throw Error(`Failed to deploy: ${deployResult.stderr}`);
  }

  await addStatus(
    BuildStatus.SUCCESS,
    "Workflow completed successfully",
    statusArray
  );
  console.log("Deployed");

  console.log("DONE Deploying, sending response");
  process.exit(0);
}
