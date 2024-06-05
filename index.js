import express from "express";
import path from "path";
import fs from "fs";
import shellExec from "shell-exec";
import os from "os";

const app = express();

app.use(express.json());

app.get("/healthcheck", (req, res) => {
  res.status(200).send("OK");
});

app.post("/deploy", async (req, res) => {
  const { body } = req;

  if (!body) {
    return res.status(400).send("Invalid request");
  }

  const { token, code } = body;

  if (!token || !code) {
    return res.status(400).send("Invalid request");
  }

  // create a temporary directory
  const tmpDir = await createTemporaryFolder();

  // get all keys of code
  await Promise.all([
    ...Object.keys(code).map(async key => {
      const codeFile = code[key];

      // get fileName from key
      const fileName = key.split("/").pop();
      const finalPath = path.join(tmpDir, ...key.split("/").slice(0, -1));
      await writeToFile(finalPath, fileName, codeFile, true);
    })
  ]);

  // deploy the code
  // npm i
  const installResult = await shellExec(`cd ${tmpDir} && npm i`).catch(e => {
    console.error("Failed to install dependencies", e);
    return null;
  });
  if (!installResult || installResult.code !== 0) {
    return res
      .status(500)
      .send(
        `Failed to install dependencies ${deployResult.stdout} ${deployResult.stderr}`
      );
  }
  const deployResult = await shellExec(
    `cd ${tmpDir} && GENEZIO_TOKEN=${token} genezio deploy`
  ).catch(e => {
    console.error("Failed to deploy", e);
    return null;
  });

  if (!deployResult || deployResult.code !== 0) {
    return res
      .status(500)
      .send(`Failed to deploy ${deployResult.stdout} ${deployResult.stderr}`);
  }

  return res.status(200).send("Deployed successfully");
});
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
    fs.writeFile(fullPath, content, function(error) {
      if (error) {
        reject(error);
        return;
      }

      resolve();
    });
  });
}

app.listen(8080, () => {
  console.log("Server running on port 8080");
});
