import { test } from "node:test";
import assert from "node:assert/strict";
import { toCSV } from "./csv.ts";

const cols = [{ key: "a", header: "a" }, { key: "b", header: "b" }];

test("quotes commas, quotes and line breaks", () => {
  assert.equal(toCSV([{ a: 'x,"y"', b: "l1\nl2\r" }], cols), 'a,b\n"x,""y""","l1\nl2\r"');
});

test("a cell a spreadsheet would run as a formula is kept as text", () => {
  // Scan data comes from the VM: a repo path or container name like this
  // must not run when the file is opened in Excel / LibreOffice.
  const rows = ["=1+1", "+cmd", "-2+3", "@SUM(A1)", "\tx", "\rx", "a=b"].map((a) => ({ a, b: "" }));
  assert.deepEqual(toCSV(rows, cols).split("\n").slice(1, 6), ["'=1+1,", "'+cmd,", "'-2+3,", "'@SUM(A1),", "'\tx,"]);
  assert.ok(toCSV(rows, cols).endsWith(`"'\rx",\na=b,`));
});

test("numbers, empty values and transforms", () => {
  assert.equal(toCSV([{ a: -5, b: null }], [...cols, { key: "c", header: "c", transform: () => "t" }]), "a,b,c\n-5,,t");
});
