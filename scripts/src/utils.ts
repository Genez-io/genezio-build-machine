import fs from "fs";
import archiver from "archiver";
import https from "https";
import axios from "axios";
import { mkdirSync } from "fs";
import { spawn } from "child_process";
import path from "path";
import os from "os";
import { parse, stringify } from "yaml-transmute";

export type StatusEntry = {
  status: string;
  message: string;
  time: string;
};

export async function zipDirectory(
  sourceDir: string,
  outPath: string,
  exclusion: string[] | undefined = undefined
) {
  const archive = archiver("zip", { zlib: { level: 9 } });
  const stream = fs.createWriteStream(outPath);

  if (!exclusion) {
    exclusion = [];
  }

  return new Promise<void>((resolve, reject) => {
    archive
      .glob("**/*", {
        cwd: sourceDir,
        dot: true,
        skip: exclusion,
      })
      .on("error", (err: Error) => reject(err))
      .pipe(stream);

    stream.on("close", () => resolve());
    archive.finalize();
  });
}

export const BuildStatus = {
  SCHEDULED: "SCHEDULED",
  AUTHENTICATING: "AUTHENTICATING",
  PULLING_CODE: "PULLING_CODE",
  CREATING_EMPTY_PROJECT: "CREATING_EMPTY_PROJECT",
  CREATING_PROJECT: "CREATING_PROJECT",
  INSTALLING_DEPS: "INSTALLING_DEPS",
  BUILDING: "BUILDING",
  DEPLOYING: "DEPLOYING",
  DEPLOYING_BACKEND: "DEPLOYING_BACKEND",
  DEPLOYING_FRONTEND: "DEPLOYING_FRONTEND",
  SUCCESS: "SUCCEEDED",
  FAILED: "FAILED",
};

async function createEmptyProject(
  token: string,
  projectName: string,
  region: string,
  stackParsed: string[] | null,
  tmpDir: string,
  stage: string
) {
  // deploy an empty project
  try {
    await axios({
      method: "PUT",
      // eslint-disable-next-line no-undef
      url: process.env.GENEZIO_API_BASE_URL + "/core/deployment",
      headers: {
        Authorization: `Bearer ${token}`,
        "Accept-Version": "genezio-cli/2.0.3",
      },
      data: {
        projectName,
        region,
        cloudProvider: "genezio-cloud",
        stage: stage,
        stack: stackParsed,
      },
    });
  } catch (e: any) {
    console.error("Failed to deploy project");
    console.log(e);
    if (
      e.response &&
      e.response.data &&
      e.response.data.error &&
      e.response.data.error.message
    ) {
      throw Error(e.response.data.error.message);
    } else {
      throw Error("Failed to deploy empty project");
    }
  }
  // get s3 presigned url
  const response = await axios({
    method: "POST",
    // eslint-disable-next-line no-undef
    url: `${process.env.GENEZIO_API_BASE_URL}/core/create-project-code-url`,
    headers: {
      Authorization: `Bearer ${token}`,
      "Accept-Version": "genezio-cli/2.0.3",
    },
    data: {
      projectName,
      region,
      stage: stage,
    },
  }).catch((e) => {
    console.log(e);
    throw Error(`Failed to create project code url ${e}`);
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
    await uploadContentToS3(url, zipdirfile);
  } catch (e) {
    console.error("Failed to upload code to S3", e);
    throw Error("Failed to upload code to S3");
  }
}

export async function addStatus(
  status: string,
  message: string,
  statusArray: StatusEntry[]
) {
  const statusFile = path.join("/tmp", "status.json");
  const newStatus = { status, message, time: new Date().toISOString() };
  statusArray.push(newStatus);
  console.log("Adding status");
  await reportBuildStatusWebHook(newStatus);
  fs.writeFile(
    statusFile,
    JSON.stringify(statusArray),
    { mode: 0o777 },
    (err) => {
      if (err) {
        console.error("Failed to write status file", err);
      }

      console.log("Wrote status file", statusFile);
    }
  );
  if (status === "FAILED") {
    // Sleep 5 seconds to allow the status file to be read
    // before the process exits
    // This is a workaround for the status file not being read in time, will be removed
    // once we properly setup state storage in a persistent database
    console.log("Sleeping for 5 seconds, waiting status read");
    await new Promise((r) => setTimeout(r, 5000));
  }
}

export async function unzipArchive(sourcePath: string, outDir: string) {
  try {
    await runNewProcessWithResult(
      "unzip",
      ["-o", sourcePath, "-d", outDir],
      path.dirname(sourcePath)
    );
  } catch (error) {
    console.error("Failed to unzip archive", error);
    throw error;
  }
}
export async function uploadContentToS3(
  presignedURL: string,
  archivePath: string
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

  return await new Promise<void>((resolve, reject) => {
    const req = https.request(options, (res) => {
      // If we don't consume the data, the "end" event will not fire
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      res.on("data", () => {});

      res.on("end", () => {
        resolve();
      });
    });

    req.on("error", (error) => {
      reject(error);
    });

    const fileStream = fs.createReadStream(archivePath);

    fileStream.on("data", () => {}).pipe(req);
  });
}

export function runNewProcessWithResult(
  command: string,
  args: string[],
  cwd = ".",
  env = {}
): Promise<{ code: number | null; stdout: string; stderr: string }> {
  return new Promise(function (resolve) {
    const child = spawn(command, args, {
      cwd,
      env: { ...process.env, ...env },
    });

    let stdout = "";
    let stderr = "";

    child.stdout.on("data", (data) => {
      stdout += data.toString();
      console.log("stdout", data.toString());
    });

    child.stderr.on("data", (data) => {
      stderr += data.toString();
      console.log("stderr", data.toString());
    });

    child.on("close", (code, signal) => {
      if (`${signal}` === "SIGSEGV") {
        code = 0;
      }
      resolve({ code, stdout, stderr });
    });

    child.on("error", (err: any) => {
      resolve({ code: err.code, stdout, stderr });
    });

    child.stdin.end();
  });
}

export async function cloneRepository(
  githubRepository: string,
  basePath: string,
  branch: string
): Promise<string> {
  // create a temporary directory
  let tmpDir = await createTemporaryFolder();
  console.log("Created temporary directory", tmpDir);

  // check if the repository and check if 200
  if (!githubRepository.includes("x-access-token")) {
    const resCheckRepo = await fetch(githubRepository).catch((e) => {
      console.error("Failed to fetch repository", e);
      return null;
    });
    if (!resCheckRepo || resCheckRepo.status !== 200) {
      throw new Error(
        "Failed to fetch the repository. It may not exist or is private"
      );
    }
  }

  // clone the repository
  console.log("Cloning repository", githubRepository, tmpDir);
  const args = branch
    ? ["clone", "--branch", branch, githubRepository, "."]
    : ["clone", githubRepository, "."];
  const cloneResult = await runNewProcessWithResult(`git`, args, tmpDir).catch(
    async (e) => {
      console.log(e);
      throw Error(`Failed to clone repository ${e}`);
    }
  );
  if (!cloneResult || cloneResult.code !== 0) {
    console.log(cloneResult);
    throw new Error(
      `Failed to clone repository ${cloneResult.stdout} ${cloneResult.stderr}`
    );
  }

  if (basePath && basePath.length > 0) {
    tmpDir = path.join(tmpDir, basePath);
  }

  return tmpDir;
}

export async function writeConfigurationFileIfNeeded(
  tmpDir: string,
  projectName: string,
  region: string
) {
  if (!fs.existsSync(path.join(tmpDir, "genezio.yaml"))) {
    // create file
    const content = `name: ${projectName}\nregion: ${region}\nyamlVersion: 2\n`;

    await writeToFile(tmpDir, "genezio.yaml", content, true).catch(
      async (e) => {
        console.error("Failed to create genezio.yaml", e);
        throw new Error("Failed to create genezio.yaml");
      }
    );
  }
}

export async function createNewProject(
  token: string,
  projectName: string,
  region: string,
  stackParsed: string[] | null,
  tmpDir: string,
  stage: string
) {
  try {
    await createEmptyProject(
      token,
      projectName,
      region,
      stackParsed,
      tmpDir,
      stage
    );
  } catch (error: any) {
    console.log(error);
    throw new Error(`Failed to create new project ${error.toString()}`);
  }
}

export async function replaceGenezioImports(
  projectName: string,
  region: string,
  tmpDir: string
) {
  try {
    if (projectName && region) {
      const yamlPath = path.join(tmpDir, "genezio.yaml");
      const yamlContent = fs.readFileSync(yamlPath, "utf-8");
      const [yaml, ctx] = parse(yamlContent);
      const _yaml: any = yaml;

      const oldYamlName = _yaml.name;
      _yaml.name = projectName;
      _yaml.region = region;

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
}

async function recursiveReplace(
  rootPath: string,
  replacements: [string, string][]
) {
  const fromStats = fs.statSync(rootPath);
  if (fromStats.isDirectory()) {
    const files = fs.readdirSync(rootPath);
    for (const file of files) {
      recursiveReplace(path.join(rootPath, file), replacements);
    }
  } else {
    const fileContent = fs.readFileSync(rootPath, "utf8");

    const newFileContent = replacements.reduce(
      (acc, [placeholder, value]) => acc.replaceAll(placeholder, value),
      fileContent
    );

    if (newFileContent !== fileContent) {
      fs.writeFileSync(rootPath, newFileContent);
    }
  }
}

export async function createTemporaryFolder(): Promise<string> {
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

    fs.mkdir(tempFolder, (error) => {
      if (error) {
        reject(error);
      }
      resolve(tempFolder);
    });
  });
}
export function writeToFile(
  folderPath: string,
  filename: string,
  content: string | Buffer,
  createPathIfNeeded = false
) {
  return new Promise<void>((resolve, reject) => {
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

export async function checkAndInstallDeps(
  currentPath: string,
  statusArray: StatusEntry[]
) {
  let shouldInstallDeps = false;

  if (
    fs.existsSync(path.join(currentPath, "next.config.js")) ||
    fs.existsSync(path.join(currentPath, "next.config.mjs"))
  ) {
    shouldInstallDeps = true;
  }

  // Check if "next" package is present in the project dependencies
  if (fs.existsSync(path.join(currentPath, "package.json"))) {
    const packageJson = JSON.parse(
      fs.readFileSync(path.join(currentPath, "package.json"), "utf-8")
    );
    if (packageJson.dependencies?.next) {
      shouldInstallDeps = true;
    }
  }

  // Check if next.config.js exists
  if (shouldInstallDeps) {
    await addStatus(
      BuildStatus.INSTALLING_DEPS,
      "Installing dependencies for next",
      statusArray
    );
    console.log("Installing dependencies for next");
    const installResult = await runNewProcessWithResult(
      `npm`,
      [`i`],
      currentPath
    ).catch(async (e) => {
      console.error("Failed to install dependencies", e);
      throw new Error(`Failed to install dependencies ${e}`);
    });
    console.log(installResult.stdout);
    console.log(installResult.stderr);
  }

  console.log("DONE Installing dependencies");

  return true;
}

export async function reportBuildStatusWebHook(newStatus: StatusEntry) {
  const reportURL = process.env.GENEZIO_API_BUILD_URL + "/report";
  if (!reportURL) {
    console.error("Missing report URL");
    return;
  }

  try {
    console.log(
      `Reporting status [${newStatus.status}:${newStatus.message}] to ${reportURL}`
    );
    await axios.post(reportURL, newStatus, {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${process.env.GENEZIO_WH_SECRET}`,
      },
    });
  } catch (e: any) {
    console.error("Failed to report status", e.code);
  }
}
