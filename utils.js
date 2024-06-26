import fs from "fs";
import archiver from "archiver";
import https from "https";

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
            res.on("data", () => {});

            res.on("end", () => {
                resolve();
            });
        });

        req.on("error", (error) => {
            reject(error);
        });

        const fileStream = fs.createReadStream(archivePath);

        fileStream
            .on("data", () => {})
            .pipe(req);
    });
}
