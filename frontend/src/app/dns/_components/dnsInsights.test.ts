import { test } from "node:test";
import assert from "node:assert/strict";
import { dnsInsights, dnsBreakdowns, isOrphan } from "./dnsInsights.ts";
import { certState } from "../../../lib/dnsCert.ts";
import type { DNSRecord } from "@/lib/types";

const t = (k: string, v?: Record<string, string>) => (v ? `${k}(${Object.values(v).join(",")})` : k);
const day = 86_400_000;
const iso = (ms: number) => new Date(Date.now() + ms).toISOString();
const rec = (p: Partial<DNSRecord>) => ({ id: 1, domain: "a.gov", has_https: true, situacao: "Ativa", tags: [], ...p }) as DNSRecord;

const records = [
  rec({ id: 1, host_ids: [7], cert_checked_at: iso(0), cert_expires_at: iso(100 * day), cert_issuer: "R3" }),
  rec({ id: 2, host_ids: [7], cert_checked_at: iso(0), cert_expires_at: iso(10 * day), cert_issuer: "R3", main_entidade: "ETIPI" }),
  rec({ id: 3, cert_checked_at: iso(0), cert_expires_at: iso(200 * day), cert_error: "x509", cert_issuer: "TRAEFIK DEFAULT CERT", services_count: 1 }),
  rec({ id: 4, has_https: false, situacao: "Em quarentena", tags: ["legado"] }),
];

test("orphan = no host, service or project", () => {
  assert.deepEqual(records.map(isOrphan), [false, false, false, true]);
});

test("insights count and carry filters", () => {
  const by = Object.fromEntries(dnsInsights(records, t, certState).map((i) => [i.key, i]));
  assert.equal(by.total.value, 4);
  assert.equal(by.total.hint, "dns.kpi.httpsCount(3)");
  assert.equal(by.certExpiring.value, 1);
  assert.deepEqual(by.certExpiring.filter, { cert: "30" });
  assert.equal(by.certError.value, 1);
  assert.equal(by.noHttps.value, 1);
  assert.deepEqual(by.noHttps.filter, { has_https: "no" });
  assert.equal(by.orphan.value, 1);
  assert.deepEqual(by["sit:Em quarentena"].filter, { situacao: "Em quarentena" });
  assert.equal(by["tag:legado"].value, 1);
});

test("breakdowns: cert buckets, issuers, hosts", () => {
  const b = dnsBreakdowns(records, certState, (id) => (id === 7 ? "web-1" : undefined));
  const cert = Object.fromEntries(b.cert.map((r) => [r.key, r.count]));
  assert.deepEqual(cert, { expired: 0, critical: 0, warning: 1, error: 1, unscanned: 0, none: 1, ok: 1 });
  assert.deepEqual(b.issuer.map((r) => [r.key, r.count]), [["R3", 2], ["TRAEFIK DEFAULT CERT", 1]]);
  assert.deepEqual(b.hosts.map((r) => [r.label, r.count]), [["web-1", 2]]);
  assert.deepEqual(b.entidade.map((r) => [r.key, r.count]), [["inventory.dash.none", 3], ["ETIPI", 1]]);
});
