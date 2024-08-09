import { runGitFlow } from "./git.js";
import { runS3Flow } from "./s3.js";
const flow = process.argv[2];

switch (flow) {
  case "s3":
    await runS3Flow();
    break;
  case "git":
    await runGitFlow();
    break;
  default:
    console.log("Invalid flow");
    process.exit(1);
    break;
}
