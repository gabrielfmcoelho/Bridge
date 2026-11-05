import { useCallback } from "react";
import { toCSV, type CSVColumn } from "@/lib/csv";

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
    const url = URL.createObjectURL(
      new Blob(["﻿" + toCSV(data, columns)], { type: "text/csv;charset=utf-8;" }),
    );
    Object.assign(document.createElement("a"), {
      href: url,
      download: `${filename}_${new Date().toISOString().slice(0, 10)}.csv`,
    }).click();
    URL.revokeObjectURL(url);
  }, [data, columns, filename]);
}
