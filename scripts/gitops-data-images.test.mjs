import assert from "node:assert/strict";
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { writeDataImages } from "./gitops-data-images.mjs";

function fixture() {
  const path = join(mkdtempSync(join(tmpdir(), "cloudsentinel-data-images-")), "kustomization.yaml");
  writeFileSync(path, `images:
  - name: cloudsentinel-mysql
    newName: old/mysql
    digest: sha256:REPLACE_MYSQL_DIGEST
  - name: cloudsentinel-redis
    newName: old/redis
    digest: sha256:REPLACE_REDIS_DIGEST
`, "utf8");
  return path;
}

test("updates both data images idempotently", () => {
  const path = fixture();
  const digests = {mysql: `sha256:${"a".repeat(64)}`, redis: `sha256:${"b".repeat(64)}`};
  writeDataImages(path, "registry.example/team/", digests);
  const first = readFileSync(path, "utf8");
  writeDataImages(path, "registry.example/team", digests);
  assert.equal(readFileSync(path, "utf8"), first);
  assert.match(first, /registry\.example\/team\/cloudsentinel-mysql/);
  assert.match(first, new RegExp(digests.redis));
});

test("rejects mutable data image references", () => {
  assert.throws(
    () => writeDataImages(fixture(), "registry.example/team", {mysql: "latest", redis: `sha256:${"b".repeat(64)}`}),
    /invalid mysql digest/,
  );
});
