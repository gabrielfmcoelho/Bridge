import { test } from "node:test";
import assert from "node:assert/strict";
import { trafficToPanel, pointsToRows, pct, ms, errorRate } from "./sentinel.ts";

test("trafficToPanel maps Sentinel traffic to the panel's stats", () => {
  const p = trafficToPanel({
    range: "24h",
    keys: ["k"],
    overview: { requests: 200, bytes_in: 0, bytes_out: 0, status: { s2xx: 180, s3xx: 0, s4xx: 8, s5xx: 12 }, latency: { p50: 20, p95: 1500, p99: 2000 }, unique_visitors: 3 },
    series: [{ bucket: 1791900000000, requests: 200, s2xx: 180, s3xx: 0, s4xx: 8, s5xx: 12, unique_visitors: 3, p95: 1500 }],
    paths: [{ path: "/tasks", app: "k", requests: 150, bytes_out: 1, s4xx: 0, s5xx: 2, p50: 10, p95: 1400 }],
  });
  assert.deepEqual(p.summary, { count: 200, users: 3, errors: 12, p50: 20, p95: 1500, p99: 2000 });
  assert.equal(p.series[0].t, new Date(1791900000000).toISOString());
  assert.equal(p.series[0].errors, 12);
  assert.deepEqual(p.top[0], { key: "/tasks", count: 150, errors: 2, p50: 10, p95: 1400, p99: 0 });
});

test("pointsToRows joins series on t, sorted", () => {
  assert.deepEqual(pointsToRows({ cpu: [{ t: 2, v: 5 }, { t: 1, v: 3 }], mem: [{ t: 1, v: 40 }] }), [
    { t: 1, cpu: 3, mem: 40 },
    { t: 2, cpu: 5 },
  ]);
});

test("formatters", () => {
  assert.equal(pct(null), "–");
  assert.equal(pct(12.345, 1), "12.3%");
  assert.equal(ms(1500), "1.5 s");
  assert.equal(ms(12.4), "12 ms");
  assert.equal(errorRate(0, 3), null);
  assert.equal(errorRate(200, 10), 5);
});
