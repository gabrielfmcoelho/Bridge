// Run: node --test src/lib/dnsCert.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { certState, certDaysLeft, matchesCertFilter, compareCertExpiry } from "./dnsCert.ts";

const DAY = 86_400_000;
const NOW = Date.parse("2026-09-22T12:00:00Z");
const at = (ms: number) => new Date(NOW + ms).toISOString();
const CHECKED = at(-DAY);

// A scanned https record expiring `ms` from NOW.
const rec = (ms: number | null, cert_error = "") => ({
  has_https: true,
  cert_expires_at: ms === null ? null : at(ms),
  cert_error,
  cert_checked_at: CHECKED,
});

test("certState: never scanned", () => {
  assert.equal(certState({ has_https: true, cert_checked_at: null }, NOW), "unscanned");
  assert.equal(certState({ has_https: false }, NOW), "none");
});

test("certState: scanned without expiry is unreachable", () => {
  assert.equal(certState(rec(null, "dial tcp: i/o timeout"), NOW), "unreachable");
});

test("certState: boundaries", () => {
  assert.equal(certState(rec(-1), NOW), "expired");
  assert.equal(certState(rec(0), NOW), "critical");
  assert.equal(certState(rec(7 * DAY - 1), NOW), "critical");
  assert.equal(certState(rec(7 * DAY), NOW), "warning");
  assert.equal(certState(rec(30 * DAY - 1), NOW), "warning");
  assert.equal(certState(rec(30 * DAY), NOW), "ok");
  assert.equal(certState(rec(30 * DAY, "x509: certificate signed by unknown authority"), NOW), "untrusted");
  // expiry outranks a verify error
  assert.equal(certState(rec(-DAY, "x509: certificate has expired"), NOW), "expired");
});

test("certDaysLeft floors", () => {
  assert.equal(certDaysLeft(at(7 * DAY), NOW), 7);
  assert.equal(certDaysLeft(at(7 * DAY - 1), NOW), 6);
  assert.equal(certDaysLeft(at(-1), NOW), -1);
});

test("matchesCertFilter: empty matches everything", () => {
  assert.equal(matchesCertFilter({ has_https: false }, "", NOW), true);
});

test("matchesCertFilter: expired is strictly before now", () => {
  assert.equal(matchesCertFilter(rec(-1), "expired", NOW), true);
  assert.equal(matchesCertFilter(rec(0), "expired", NOW), false);
  assert.equal(matchesCertFilter(rec(null), "expired", NOW), false);
});

test("matchesCertFilter: 7 is [now, now+7d)", () => {
  assert.equal(matchesCertFilter(rec(-1), "7", NOW), false);
  assert.equal(matchesCertFilter(rec(0), "7", NOW), true);
  assert.equal(matchesCertFilter(rec(7 * DAY - 1), "7", NOW), true);
  assert.equal(matchesCertFilter(rec(7 * DAY), "7", NOW), false);
  assert.equal(matchesCertFilter(rec(null), "7", NOW), false);
});

test("matchesCertFilter: 30 is [now, now+30d) and includes the 7-day ones", () => {
  assert.equal(matchesCertFilter(rec(-1), "30", NOW), false);
  assert.equal(matchesCertFilter(rec(0), "30", NOW), true);
  assert.equal(matchesCertFilter(rec(3 * DAY), "30", NOW), true);
  assert.equal(matchesCertFilter(rec(30 * DAY - 1), "30", NOW), true);
  assert.equal(matchesCertFilter(rec(30 * DAY), "30", NOW), false);
  assert.equal(matchesCertFilter(rec(null), "30", NOW), false);
});

test("matchesCertFilter: error", () => {
  assert.equal(matchesCertFilter(rec(null, "refused"), "error", NOW), true);
  assert.equal(matchesCertFilter(rec(90 * DAY), "error", NOW), false);
});

test("matchesCertFilter: unscanned is https-only and never checked", () => {
  assert.equal(matchesCertFilter({ has_https: true, cert_checked_at: null }, "unscanned", NOW), true);
  assert.equal(matchesCertFilter({ has_https: false, cert_checked_at: null }, "unscanned", NOW), false);
  assert.equal(matchesCertFilter(rec(90 * DAY), "unscanned", NOW), false);
});

test("compareCertExpiry: nulls last in both directions", () => {
  const soon = rec(DAY), late = rec(90 * DAY), none = rec(null), never = { has_https: true, cert_expires_at: null, cert_error: "", cert_checked_at: null };
  const sort = (dir: "asc" | "desc") =>
    [none, late, never, soon].sort((a, b) => compareCertExpiry(a, b, dir)).map((d) => d.cert_expires_at);
  assert.deepEqual(sort("asc"), [soon.cert_expires_at, late.cert_expires_at, null, null]);
  assert.deepEqual(sort("desc"), [late.cert_expires_at, soon.cert_expires_at, null, null]);
});
