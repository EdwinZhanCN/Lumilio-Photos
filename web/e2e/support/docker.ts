import path from "node:path";
import { fileURLToPath } from "node:url";

export const repositoryRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);

const extraFile = process.env.LUMILIO_E2E_COMPOSE_EXTRA;
const extraPath = extraFile
  ? path.isAbsolute(extraFile)
    ? extraFile
    : path.resolve(repositoryRoot, extraFile)
  : undefined;

export const docker = process.env.LUMILIO_E2E_DOCKER_HOST
  ? ["--host", process.env.LUMILIO_E2E_DOCKER_HOST]
  : [];

export const compose = [
  "compose",
  "-f",
  path.join(repositoryRoot, "web/e2e/compose.yml"),
  ...(extraPath ? ["-f", extraPath] : []),
  "-p",
  "lumilio-photos-e2e",
];
