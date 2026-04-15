import { useCallback } from "react";

interface CSVColumn<T> {
  key: string;
  header: string;
  transform?: (item: T) => string;
}

/**
 * Returns a callback that exports an array of items as a CSV file.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useExportCSV<T extends Record<string, any>>(
  data: T[],
  columns: CSVColumn<T>[],
  filename: string,
): () => void {
  return useCallback(() => {
    if (data.length === 0) return;

    const escape = (v: unknown) => {
      const s = String(v ?? "");
      return /[,"\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
    };

    const headerRow = columns.map((c) => c.header).join(",");
    const rows = data.map((item) =>
      columns
        .map((col) =>
          escape(col.transform ? col.transform(item) : item[col.key]),
        )
        .join(","),
    );

    const csv = [headerRow, ...rows].join("\n");
    const url = URL.createObjectURL(
      new Blob(["\uFEFF" + csv], { type: "text/csv;charset=utf-8;" }),
    );
    Object.assign(document.createElement("a"), {
      href: url,
      download: `${filename}_${new Date().toISOString().slice(0, 10)}.csv`,
    }).click();
    URL.revokeObjectURL(url);
  }, [data, columns, filename]);
}
