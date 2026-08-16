import fs from "node:fs";

const args = new Map();
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index];
  const value = process.argv[index + 1];
  if (!key?.startsWith("--") || value === undefined) {
    throw new Error(`invalid argument near ${key ?? "<end>"}`);
  }
  args.set(key.slice(2), value);
}

const file = args.get("file");
const registry = args.get("registry");
if (!file || !registry) {
  throw new Error("--file and --registry are required");
}

const images = [
  ["prometheus", "cloudsentinel-prometheus", "MONITORING_PROMETHEUS_DIGEST"],
  ["alertmanager", "cloudsentinel-alertmanager", "MONITORING_ALERTMANAGER_DIGEST"],
  ["grafana", "cloudsentinel-grafana", "MONITORING_GRAFANA_DIGEST"],
  ["metrics-server", "cloudsentinel-metrics-server", "MONITORING_METRICS_SERVER_DIGEST"],
];

let content = fs.readFileSync(file, "utf8");
for (const [argument, image, placeholder] of images) {
  const digest = args.get(`${argument}-digest`);
  if (!/^sha256:[0-9a-f]{64}$/.test(digest ?? "")) {
    throw new Error(`invalid --${argument}-digest`);
  }

  const block = new RegExp(
    `(\\n  - name: ${image}\\n` +
      `    newName: )[^\\n]+(\\n` +
      `    digest: sha256:)(?:${placeholder}|[0-9a-f]{64})(\\n)`,
    "g",
  );
  const matches = [...content.matchAll(block)];
  if (matches.length !== 1) {
    throw new Error(`expected exactly one image block for ${image}`);
  }
  content = content.replace(
    block,
    `$1${registry}/${image}$2${digest.slice("sha256:".length)}$3`,
  );
}

if (/MONITORING_[A-Z_]+_DIGEST/.test(content)) {
  throw new Error("monitoring image placeholders remain after update");
}

fs.writeFileSync(file, content, "utf8");

