"use client";

import { useEffect, useId, useSyncExternalStore } from "react";
import type { Host } from "@/lib/types";

// One quick-look panel for the whole hosts list. Any card or table can open it;
// exactly one mounted <QuickLookOutlet> renders it: the first to register. If
// that one unmounts (filtered out, page change), the next registered takes over.
// ponytail: mounting <HostQuickLookRoot/> once in hosts/page.tsx would replace
// the renderer election (`renderers`) — do that when page.tsx is free to edit.

type State = { host: Host | null; open: boolean };

let state: State = { host: null, open: false };
let renderers: string[] = [];
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());
const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

export function openQuickLook(host: Host) {
  state = { host, open: true };
  emit();
}

// The host stays in state after close so the panel slides out with its content.
export function closeQuickLook() {
  if (!state.open) return;
  state = { ...state, open: false };
  emit();
}

export function useQuickLook(): State {
  return useSyncExternalStore(subscribe, () => state, () => state);
}

/** True for the one outlet elected to render the panel. */
export function useIsQuickLookRenderer(): boolean {
  const id = useId();
  useEffect(() => {
    renderers.push(id);
    emit();
    return () => {
      renderers = renderers.filter((r) => r !== id);
      emit();
    };
  }, [id]);
  return useSyncExternalStore(subscribe, () => renderers[0] === id, () => false);
}
