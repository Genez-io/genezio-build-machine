import { addStatus, BuildStatus, runNewProcessWithResult, StatusEntry, unzipArchive, } from "./utils.js";

type InputParams = {
    token: string;
    stage: string;
};

function parseArguments(): InputParams {
    const token = process.argv[3];
    const stage = process.argv[4];
    if (!token) {
        throw new Error("Token is required");
    }
    if (!stage) {
        throw new Error("Stage is required");
    }
    return { token, stage };
}

export async function runS3Flow() {
    console.log("Starting build from s3 flow");

    const args = parseArguments();
    const statusArray: StatusEntry[] = []
    try {
        await deployFromArchive(args, statusArray);
    } catch (e: any) {
        console.log(e)
        await addStatus(BuildStatus.FAILED, e.toString(), statusArray);
    }
}

async function deployFromArchive(params: InputParams, statusArray: StatusEntry[] = []) {
    const tmpDir = "/tmp/projectCode"
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
        "npm", ["i"],
        tmpDir
    ).catch(e => {
        throw Error(`Failed to install dependencies ${e}`);
    });

    if (!installResult || installResult.code !== 0) {
        console.log(installResult.stdout)
        console.log(installResult.stderr)
        throw Error(`Failed to install dependencies: ${installResult.stderr}`);
    }

    console.log("Installed dependencies");
    await addStatus(BuildStatus.AUTHENTICATING, "Authenticating with genezio", statusArray);
    const loginResult = await runNewProcessWithResult(
        "genezio", ["login", token],
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
        "genezio", ["deploy", "--stage", params.stage],
        tmpDir,
        { CI: "true" }
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


