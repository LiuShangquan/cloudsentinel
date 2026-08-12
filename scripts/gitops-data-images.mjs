#!/usr/bin/env node

import { readFileSync, writeFileSync } from "node:fs";

const components = ["mysql", "redis"];
const digestPattern = /^sha256:[0-9a-f]{64}$/;

export function writeDataImages(path, registry, digests) {
  const normalizedRegistry = registry.replace(/\/+$/, "");
  for (const component of components) {
    if (!digestPattern.test(digests[component] ?? "")) {
      throw new Error(`invalid ${component} digest: ${digests[component] ?? "missing"}`);
    }
  }

  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  let active;
  const seen = new Set();
  const output = lines.map((line) => {
    const match = line.match(/^\s*- name: cloudsentinel-(mysql|redis)\s*$/);
    if (match) {
      active = match[1];
      seen.add(active);
      return line;
    }
    if (active && line.includes("newName:")) {
      return `${line.match(/^\s*/)[0]}newName: ${normalizedRegistry}/cloudsentinel-${active}`;
    }
    if (active && line.includes("digest:")) {
      const updated = `${line.match(/^\s*/)[0]}digest: ${digests[active]}`;
      active = undefined;
      return updated;
    }
    return line;
  });

  const missing = components.filter((component) => !seen.has(component));
  if (missing.length) throw new Error(`${path}: missing image blocks: ${missing.join(", ")}`);
  writeFileSync(path, `${output.join("\n").replace(/\n+$/, "")}\n`, "utf8");
}

function parseArgs(values) {
  const args = {};
  for (let index = 0; index < values.length; index += 2) {
    const key = values[index];
    const value = values[index + 1];
    if (!key?.startsWith("--") || value === undefined) throw new Error(`invalid argument: ${key ?? "missing"}`);
    args[key.slice(2)] = value;
  }
  return args;
}

export function main(values) {
  const args = parseArgs(values);
  for (const key of ["file", "registry", "mysql-digest", "redis-digest"]) {
    if (!args[key]) throw new Error(`--${key} is required`);
  }
  writeDataImages(args.file, args.registry, {
    mysql: args["mysql-digest"],
    redis: args["redis-digest"],
  });
}

if (process.argv[1] && import.meta.url === new URL(`file://${process.argv[1].replace(/\\/g, "/")}`).href) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
