import { runNewProcessWithResult, cloneRepository, addStatus, BuildStatus, writeConfigurationFileIfNeeded, createNewProject, replaceGenezioImports, StatusEntry, checkAndInstallDeps } from "./utils.js";

type InputParams = {
    token: string;
    githubRepository: string;
    projectName: string;
    region: string;
    basePath: string;
    isNewProject: boolean;
    stack: string[] | null;
    stage: string;
}

export async function runGitFlow() {
    console.log("Starting build from git flow");

    const input = parseArguments();
    const statusArray: StatusEntry[] = []
    try {
        await deployFromGit(input);
    } catch (e: any) {
        await addStatus(BuildStatus.FAILED, e.toString(), statusArray);
    }
}

function parseArguments(): InputParams {
    console.log(process.argv)
    const token = process.argv[3];
    const githubRepository = process.argv[4];
    const projectName = process.argv[5];
    const region = process.argv[6];
    const basePath = process.argv[7];
    let stack = null;

    try {
        stack = JSON.parse(process.argv[8]);
    } catch (e) {
        console.log("Stack does not exist")
    }
    const isNewProject = process.argv[9] === "true";
    const stage = process.argv[10];

    return {
        token, githubRepository, projectName, region, basePath, isNewProject, stack, stage
    }
}

async function deployFromGit(params: InputParams, statusArray: StatusEntry[] = []) {
    await addStatus(BuildStatus.PENDING, "Starting build from git flow", statusArray);

    let { token, githubRepository, projectName, region, basePath, isNewProject, stack, stage } = params;
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
        throw Error(`Authenticating failed: ${loginResult.stderr}`);
    }
    console.log("Logged in");

    await addStatus(BuildStatus.PULLING_CODE, "Pulling code from github", statusArray);

    const folder = await cloneRepository(githubRepository, basePath, stage);
    await writeConfigurationFileIfNeeded(folder, projectName, region);

    if (stage === "main" || stage === "master" || stage === "") {
        stage = "prod";
    }

    await addStatus(BuildStatus.CREATING_PROJECT, "Creating project", statusArray);

    if (isNewProject) {
        await addStatus(BuildStatus.CREATING_PROJECT, "Creating new project", statusArray);
        await createNewProject(token, projectName, region, stack, folder, stage);
    }

    console.log("Installing dependencies if needed...");
    await checkAndInstallDeps(folder, statusArray);

    await replaceGenezioImports(projectName, region, folder)

    // deploy the code
    console.log("Deploying...");
    await addStatus(BuildStatus.DEPLOYING, "Deploying project", statusArray);
    const deployResult = await runNewProcessWithResult(
        `genezio`,
        [`deploy`, `--stage`, `${stage}`],
        folder,
        {
            "CI": true
        }
    ).catch(async e => {
        throw Error(`Failed to deploy ${e}`);
    });

    console.log(deployResult);
    if (!deployResult || deployResult.code !== 0) {
        throw Error(`Failed to deploy: ${deployResult.stderr}`);
    }

    await addStatus(BuildStatus.SUCCESS, "Workflow completed successfully", statusArray);
    console.log("Deployed");

    console.log("DONE Deploying, sending response");
    process.exit(0);
}

