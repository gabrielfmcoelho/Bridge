"use client";

import { useCallback, useRef, useState } from "react";

export type BatchStatus = "pending" | "running" | "success" | "failed";
export type BatchProgress = Record<string, { status: BatchStatus; error?: string; attempt?: number }>;

export interface BatchRunOptions<H extends { oficial_slug: string }> {
  hosts: H[];
  concurrency: number;
  maxAttempts?: number;
  retryBackoffMs?: number[];
  runOne: (host: H) => Promise<{ success: boolean; error?: string }>;
}

// Sliding-window worker pool that mirrors the host scan flow: workers share a
// single cursor so faster hosts free up slots immediately. Stop is cooperative
// — in-flight requests are not aborted, only new hosts are skipped.
export function useBatchRunner() {
  const [running, setRunning] = useState(false);
  const [progress, setProgress] = useState<BatchProgress>({});
  const abortRef = useRef(false);

  const stop = useCallback(() => { abortRef.current = true; }, []);

  const start = useCallback(async <H extends { oficial_slug: string }>(opts: BatchRunOptions<H>) => {
    const { hosts, concurrency, maxAttempts = 1, retryBackoffMs = [], runOne } = opts;
    if (hosts.length === 0) return;
    abortRef.current = false;
    setRunning(true);
    const initial: BatchProgress = {};
    hosts.forEach(h => { initial[h.oficial_slug] = { status: "pending" }; });
    setProgress(initial);

    let cursor = 0;
    const workerCount = Math.min(Math.max(1, concurrency), hosts.length);
    const workers = Array.from({ length: workerCount }, async () => {
      while (true) {
        if (abortRef.current) return;
        const i = cursor++;
        if (i >= hosts.length) return;
        const host = hosts[i];

        let lastError = "Operation failed";
        let succeeded = false;
        for (let attempt = 1; attempt <= maxAttempts; attempt++) {
          if (abortRef.current) return;
          setProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "running", attempt } }));
          try {
            const res = await runOne(host);
            if (res.success) {
              setProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "success", attempt } }));
              succeeded = true;
              break;
            }
            lastError = res.error || "Operation failed";
          } catch (err: unknown) {
            lastError = err instanceof Error ? err.message : "Unknown error";
          }
          if (attempt < maxAttempts) {
            await new Promise(r => setTimeout(r, retryBackoffMs[attempt - 1] ?? 1000));
          }
        }
        if (!succeeded) {
          setProgress(prev => ({ ...prev, [host.oficial_slug]: { status: "failed", error: lastError, attempt: maxAttempts } }));
        }
      }
    });
    await Promise.all(workers);
    setRunning(false);
  }, []);

  const counts = Object.values(progress);
  const doneCount = counts.filter(s => s.status === "success" || s.status === "failed").length;
  const successCount = counts.filter(s => s.status === "success").length;
  const failedCount = counts.filter(s => s.status === "failed").length;

  return { running, progress, doneCount, successCount, failedCount, start, stop };
}
