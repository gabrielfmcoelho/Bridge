import { test } from "node:test";
import assert from "node:assert/strict";
import type { WebServer } from "./api.ts";
import { linkableName, linkedIndex, nameStatus, poolUsers, undeclaredRecords } from "./webDns.ts";

test("linkableName mirrors the Go filter", () => {
  assert.equal(linkableName("Portal.SEAD.pi.gov.br."), "portal.sead.pi.gov.br");
  for (const n of ["_", "localhost", "app.localhost", "10.0.0.1", "::1", "*.x.gov.br", "~^(?<s>.+)$", "intranet", "a b.com", ""]) {
    assert.equal(linkableName(n), null, n);
  }
});

const servers: WebServer[] = [
  { kind: "nginx", sites: [
    { names: ["Known.x.gov.br", "_"], fastcgi: ["unix:/run/php/php8.1-fpm.sock"] },
    { names: ["app.x.gov.br"], fastcgi: ["php:9000"] },
  ] },
  { kind: "php-fpm", container: "php", pools: [{ name: "www", listen: "9000" }] },
];

test("nameStatus: linked by domain (any case), unlinked, ignored", () => {
  const idx = linkedIndex([{ id: 7, domain: "known.x.gov.br" }]);
  assert.deepEqual(nameStatus("Known.x.gov.br", idx), { kind: "linked", dnsId: 7 });
  assert.deepEqual(nameStatus("app.x.gov.br", idx), { kind: "unlinked" });
  assert.deepEqual(nameStatus("_", idx), { kind: "ignored" });
});

test("undeclaredRecords: linked records no server declares", () => {
  const recs = [{ id: 1, domain: "known.x.gov.br" }, { id: 2, domain: "old.x.gov.br" }];
  assert.deepEqual(undeclaredRecords(recs, servers).map((r) => r.id), [2]);
});

test("poolUsers: by socket path and by port", () => {
  assert.deepEqual(poolUsers({ name: "www", listen: "9000" }, servers), ["app.x.gov.br"]);
  assert.deepEqual(poolUsers({ name: "www", listen: "/run/php/php8.1-fpm.sock" }, servers), ["Known.x.gov.br"]);
  assert.deepEqual(poolUsers({ name: "www", listen: "127.0.0.1:9001" }, servers), []);
});
