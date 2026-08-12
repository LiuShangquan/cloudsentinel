#!/usr/bin/env node

import { readFileSync, writeFileSync } from "node:fs";

export const components = ["api", "worker", "migrate"];
const digestPattern = /^sha256:[0-9a-f]{64}$/;

export function readImages(path) {
  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  const images = {};
  for (let index = 0; index < lines.length; index += 1) {
    const match = lines[index].match(/^\s*- name: cloudsentinel-(api|worker|migrate)\s*$/);
    if (!match) continue;
    const component = match[1];
    const block = lines.slice(index + 1, index + 4);
    const nameLine = block.find((line) => line.includes("newName:"));
    const digestLine = block.find((line) => line.includes("digest:"));
    if (!nameLine || !digestLine) throw new Error(`${path}: incomplete image block for ${component}`);
    images[component] = {
      name: nameLine.split("newName:", 2)[1].trim(),
      digest: digestLine.split("digest:", 2)[1].trim(),
    };
  }
  const missing = components.filter((component) => !images[component]);
  if (missing.length) throw new Error(`${path}: missing image blocks: ${missing.join(", ")}`);
  return images;
}

export function writeImages(path, images) {
  for (const component of components) {
    if (!images[component] || !digestPattern.test(images[component].digest)) {
      throw new Error(`invalid ${component} digest: ${images[component]?.digest ?? "missing"}`);
    }
  }
  const lines = readFileSync(path, "utf8").split(/\r?\n/);
  let active;
  const seen = new Set();
  const output = lines.map((line) => {
    const match = line.match(/^\s*- name: cloudsentinel-(api|worker|migrate)\s*$/);
    if (match) {
      active = match[1];
      seen.add(active);
      return line;
    }
    if (active && line.includes("newName:")) {
      return `${line.match(/^\s*/)[0]}newName: ${images[active].name}`;
    }
    if (active && line.includes("digest:")) {
      const updated = `${line.match(/^\s*/)[0]}digest: ${images[active].digest}`;
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
  if (!args.file) throw new Error("--file is required");
  let images;
  if (args["copy-from"]) {
    const explicit = ["registry", "api-digest", "worker-digest", "migrate-digest"].some((key) => args[key]);
    if (explicit) throw new Error("--copy-from cannot be combined with explicit image values");
    images = readImages(args["copy-from"]);
  } else {
    for (const key of ["registry", "api-digest", "worker-digest", "migrate-digest"]) {
      if (!args[key]) throw new Error(`--${key} is required`);
    }
    const registry = args.registry.replace(/\/+$/, "");
    images = Object.fromEntries(components.map((component) => [component, {
      name: `${registry}/cloudsentinel-${component}`,
      digest: args[`${component}-digest`],
    }]));
  }
  writeImages(args.file, images);
}

if (process.argv[1] && import.meta.url === new URL(`file://${process.argv[1].replace(/\\/g, "/")}`).href) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}
