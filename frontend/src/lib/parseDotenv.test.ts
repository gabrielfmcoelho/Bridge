// Run: node --test src/lib/parseDotenv.test.ts   (Node 24 native TS, no runner dep)
import { test } from "node:test";
import assert from "node:assert/strict";
import { parseDotenv } from "./parseDotenv.ts";

test("parseDotenv covers the .env shapes we accept", () => {
  const cases: { in: string; want: { name: string; value: string; description: string }[] }[] = [
    // plain
    { in: "KEY=value", want: [{ name: "KEY", value: "value", description: "" }] },
    // inline comment -> description (space before #)
    { in: "KEY=value #note", want: [{ name: "KEY", value: "value", description: "note" }] },
    // export prefix stripped
    { in: "export KEY=value", want: [{ name: "KEY", value: "value", description: "" }] },
    // hash without leading space stays in value (password survives)
    { in: "KEY=pa#ss", want: [{ name: "KEY", value: "pa#ss", description: "" }] },
    // double-quoted value: inner # is literal, unquoted
    { in: 'KEY="a #b"', want: [{ name: "KEY", value: "a #b", description: "" }] },
    // single-quoted value
    { in: "KEY='a #b'", want: [{ name: "KEY", value: "a #b", description: "" }] },
    // comment after a quoted value
    { in: 'KEY="v" # note', want: [{ name: "KEY", value: "v", description: "note" }] },
    // lowercase key upper-cased; whitespace around key/value trimmed
    { in: "  key  =  value  ", want: [{ name: "KEY", value: "value", description: "" }] },
    // first '=' wins; later '=' stays in value
    { in: "KEY=a=b", want: [{ name: "KEY", value: "a=b", description: "" }] },
  ];
  for (const c of cases) {
    assert.deepEqual(parseDotenv(c.in), c.want, `input: ${JSON.stringify(c.in)}`);
  }
});

test("parseDotenv skips blanks, full-line comments, and no-'=' lines; parses multi-line", () => {
  const input = [
    "# a full-line comment",
    "",
    "   ",
    "DB_URL=postgres://x #primary",
    "noequalsline",
    "API_KEY=k-123",
  ].join("\n");
  assert.deepEqual(parseDotenv(input), [
    { name: "DB_URL", value: "postgres://x", description: "primary" },
    { name: "API_KEY", value: "k-123", description: "" },
  ]);
});
