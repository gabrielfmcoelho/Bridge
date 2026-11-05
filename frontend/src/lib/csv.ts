// CSV text for the list exports (useExportCSV). Runtime-import free so
// `node --test` can run csv.test.ts.

export interface CSVColumn<T> {
  key: string;
  header: string;
  transform?: (item: T) => string;
}

// A text cell starting with one of these runs as a formula in Excel /
// LibreOffice; a leading ' keeps it text. Exports carry data Bridge doesn't
// control (scan results come from the VM), so every text cell is checked.
const FORMULA_START = /^[=+\-@\t\r]/;

function cell(v: unknown): string {
  let s = String(v ?? "");
  if (typeof v === "string" && FORMULA_START.test(s)) s = `'${s}`;
  return /[,"\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function toCSV<T extends Record<string, any>>(data: T[], columns: CSVColumn<T>[]): string {
  const rows = data.map((item) => columns.map((c) => cell(c.transform ? c.transform(item) : item[c.key])).join(","));
  return [columns.map((c) => c.header).join(","), ...rows].join("\n");
}
