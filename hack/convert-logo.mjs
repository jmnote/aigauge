import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { Resvg } from "@resvg/resvg-js";

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = path.join(repo, "frontend", "logo.svg");
const output = path.join(repo, "frontend", "logo.png");
const png = new Resvg(fs.readFileSync(source), {
  fitTo: { mode: "original" },
}).render().asPng();

fs.writeFileSync(output, png);
console.log(`Created: ${path.relative(repo, output)}`);
