"use client";

import { useCallback, useEffect, useRef, useState } from "react";

// Copy-to-clipboard with the 2 s "copied" flash every site was re-implementing.
// Resolves false when the Clipboard API is unavailable (http:// on the LAN) so
// callers can fall back to a selectable <pre>.
export function useCopy(resetMs = 2000) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  const copy = useCallback(async (value: string) => {
    try {
      await navigator.clipboard.writeText(value);
    } catch {
      return false;
    }
    setCopied(true);
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => setCopied(false), resetMs);
    return true;
  }, [resetMs]);

  return { copied, copy };
}
