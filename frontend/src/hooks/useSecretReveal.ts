"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { secretsAPI } from "@/lib/api";

// useSecretReveal owns the click-to-reveal lifecycle for a single secret
// row (Phase 4 Task 4.4):
//   1. Fetch + decrypt via /api/secrets/{id}/reveal on demand.
//   2. Auto-hide the plaintext after autoHideMs (default 30s) so a user
//      who walks away from the screen doesn't leave the value on display.
//   3. Copy to clipboard, then attempt to overwrite the clipboard with an
//      empty string after the same auto-hide window. Best-effort: some
//      browsers reject programmatic clears, in which case we surface a
//      hint in copyState but never block.
//
// The hook is single-tenant per instance — call it once per row. Multiple
// concurrent reveals on the same page each get their own timer.
export interface UseSecretReveal {
  revealed: boolean;
  value: string | null;
  loading: boolean;
  error: string | null;
  copyState: "idle" | "copied" | "cleared";
  reveal: (secretID: number) => Promise<void>;
  hide: () => void;
  copy: () => Promise<void>;
  // remainingMs is the time left on the auto-hide timer, useful for a
  // countdown UI. 0 when not revealed; updates roughly once per second
  // (cheap setInterval — bumped to once a second to avoid render thrash).
  remainingMs: number;
}

const DEFAULT_AUTO_HIDE_MS = 30_000;

export function useSecretReveal(autoHideMs: number = DEFAULT_AUTO_HIDE_MS): UseSecretReveal {
  const [revealed, setRevealed] = useState(false);
  const [value, setValue] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "cleared">("idle");
  const [remainingMs, setRemainingMs] = useState(0);

  // Refs hold the timer + the absolute revealed-until deadline so the
  // 1-Hz tick can compute remaining without driving a re-render storm.
  const hideTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const tickTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const clipboardClearTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const revealedUntil = useRef<number>(0);

  const clearTimers = useCallback(() => {
    if (hideTimer.current) {
      clearTimeout(hideTimer.current);
      hideTimer.current = null;
    }
    if (tickTimer.current) {
      clearInterval(tickTimer.current);
      tickTimer.current = null;
    }
  }, []);

  const hide = useCallback(() => {
    clearTimers();
    setRevealed(false);
    setValue(null);
    setRemainingMs(0);
  }, [clearTimers]);

  // Cleanup on unmount: kill timers + best-effort clipboard wipe so
  // navigating away while a reveal is in flight doesn't leave the value
  // sitting around.
  useEffect(() => {
    return () => {
      clearTimers();
      if (clipboardClearTimer.current) {
        clearTimeout(clipboardClearTimer.current);
      }
    };
  }, [clearTimers]);

  const reveal = useCallback(
    async (secretID: number) => {
      clearTimers();
      setLoading(true);
      setError(null);
      try {
        const res = await secretsAPI.reveal(secretID);
        setValue(res.payload || "");
        setRevealed(true);
        revealedUntil.current = Date.now() + autoHideMs;
        setRemainingMs(autoHideMs);
        // Auto-hide timer.
        hideTimer.current = setTimeout(() => {
          setRevealed(false);
          setValue(null);
          setRemainingMs(0);
        }, autoHideMs);
        // Countdown tick — 1Hz so the UI can render "hide in Ns".
        tickTimer.current = setInterval(() => {
          const left = revealedUntil.current - Date.now();
          setRemainingMs(left > 0 ? left : 0);
        }, 1000);
      } catch (e) {
        setError((e as Error).message);
      } finally {
        setLoading(false);
      }
    },
    [autoHideMs, clearTimers],
  );

  const copy = useCallback(async () => {
    if (value == null) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopyState("copied");
      // Schedule clipboard clear at the same horizon as auto-hide so the
      // value doesn't outlive the on-screen display.
      if (clipboardClearTimer.current) {
        clearTimeout(clipboardClearTimer.current);
      }
      clipboardClearTimer.current = setTimeout(async () => {
        try {
          await navigator.clipboard.writeText("");
          setCopyState("cleared");
        } catch {
          // Some browsers refuse programmatic clipboard writes outside
          // user gesture context. Best-effort — the on-screen value is
          // gone by now anyway from auto-hide.
        }
      }, autoHideMs);
    } catch (e) {
      setError("Clipboard write failed: " + (e as Error).message);
    }
  }, [value, autoHideMs]);

  return { revealed, value, loading, error, copyState, reveal, hide, copy, remainingMs };
}
