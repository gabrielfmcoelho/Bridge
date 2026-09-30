// Run: node --test src/lib/secretFields.test.ts
import { test } from "node:test";
import assert from "node:assert/strict";
import { secretFields } from "./secretFields.ts";

test("app login becomes labelled fields in order, empty ones dropped", () => {
  const f = secretFields("app_login", JSON.stringify({ app_name: "Airflow", username: "airflow", password: "p", url: "http://a/", notes: "" }));
  assert.deepEqual(f.map((x) => [x.key, x.kind]), [["app_name", "text"], ["url", "url"], ["username", "text"], ["password", "secret"]]);
});

test("passwords: wrapped or raw", () => {
  assert.deepEqual(secretFields("password", '{"value":"s3"}'), [{ key: "value", value: "s3", kind: "secret" }]);
  assert.deepEqual(secretFields("password", "raw-pw"), [{ key: "value", value: "raw-pw", kind: "secret" }]);
});

test("unknown keys still show; broken JSON shows as text", () => {
  const f = secretFields("cred", JSON.stringify({ username: "u", password: "p", realm: "AD" }));
  assert.deepEqual(f.map((x) => x.key), ["username", "password", "realm"]);
  assert.equal(secretFields("sshkey", "{oops")[0].value, "{oops");
});
