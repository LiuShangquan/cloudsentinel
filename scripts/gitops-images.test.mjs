import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { readImages, writeImages } from "./gitops-images.mjs";

const sample = `images:
  - name: cloudsentinel-api
    newName: old/api
    digest: sha256:REPLACE_API_DIGEST
  - name: cloudsentinel-worker
    newName: old/worker
    digest: sha256:REPLACE_WORKER_DIGEST
  - name: cloudsentinel-migrate
    newName: old/migrate
    digest: sha256:REPLACE_MIGRATE_DIGEST
  - name: cloudsentinel-web
    newName: old/web
    digest: sha256:REPLACE_WEB_DIGEST
`;

function fixture() {
  const path = join(mkdtempSync(join(tmpdir(), "cloudsentinel-gitops-")), "kustomization.yaml");
  writeFileSync(path, sample, "utf8");
  return path;
}

test("writes and reads immutable image references", () => {
  const digest = `sha256:${"a".repeat(64)}`;
  const images = Object.fromEntries(["api", "worker", "migrate", "web"].map((component) => [component, {
    name: `registry/${component}`,
    digest,
  }]));
  const path = fixture();
  writeImages(path, images);
  assert.deepEqual(readImages(path), images);
  assert.match(readFileSync(path, "utf8"), /registry\/worker/);
});

test("rejects mutable or malformed digests", () => {
  const images = Object.fromEntries(["api", "worker", "migrate", "web"].map((component) => [component, {
    name: `registry/${component}`,
    digest: "latest",
  }]));
  assert.throws(() => writeImages(fixture(), images), /invalid api digest/);
});
