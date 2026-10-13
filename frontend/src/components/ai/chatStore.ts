"use client";

import { useEffect, useSyncExternalStore } from "react";
import { aiAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { historyFor, type ChatMsg, type ProposalState } from "./chatModel";

export type { ChatMsg, ProposalState };

// The assistant conversation, shared by the header drawer and the home
// widget: one thread per browser tab (sessionStorage), so it survives
// navigation and reload, and a "continue in the assistant" opens the same
// thread. Module state + useSyncExternalStore; no provider needed.

interface ChatState {
  owner: number | null;
  messages: ChatMsg[];
  pending: boolean;
  /** The header drawer is open. */
  drawerOpen: boolean;
}

const KEY = "bridge.assistant.thread";
const EMPTY: ChatState = { owner: null, messages: [], pending: false, drawerOpen: false };

function load(): ChatState {
  try {
    const saved = JSON.parse(sessionStorage.getItem(KEY) ?? "null") as { owner: number; messages: ChatMsg[] } | null;
    return saved ? { ...EMPTY, owner: saved.owner, messages: saved.messages } : EMPTY;
  } catch {
    return EMPTY; // server render, private mode, corrupt entry
  }
}

let state: ChatState = typeof window === "undefined" ? EMPTY : load();
const listeners = new Set<() => void>();
let inflight: AbortController | null = null;
let seq = 0;

function set(patch: Partial<ChatState>) {
  state = { ...state, ...patch };
  try {
    sessionStorage.setItem(KEY, JSON.stringify({ owner: state.owner, messages: state.messages }));
  } catch { /* per-tab convenience only */ }
  listeners.forEach((l) => l());
}

const msg = (role: ChatMsg["role"], content: string, extra?: Partial<ChatMsg>): ChatMsg =>
  ({ id: `${Date.now().toString(36)}-${seq++}`, role, content, ...extra });

async function send(text: string) {
  const content = text.trim();
  if (!content || state.pending) return;
  const history = historyFor(state.messages);
  set({ messages: [...state.messages, msg("user", content)], pending: true });
  const ctrl = new AbortController();
  inflight = ctrl;
  try {
    const res = await aiAPI.chat(content, history, ctrl.signal);
    if (!ctrl.signal.aborted) {
      set({ messages: [...state.messages, msg("assistant", res.response, {
        steps: res.steps,
        refs: res.refs,
        proposals: res.proposals?.map((p) => ({ ...p, status: "pending" as const })),
        noTools: res.tools === false,
      })] });
    }
  } catch (err) {
    if (!ctrl.signal.aborted) set({ messages: [...state.messages, msg("assistant", err instanceof Error ? err.message : "", { error: true })] });
  } finally {
    if (inflight === ctrl) {
      inflight = null;
      set({ pending: false });
    }
  }
}

/** Cancel the request in flight; the question stays in the thread. */
function stop() {
  inflight?.abort();
  inflight = null;
  if (state.pending) set({ pending: false });
}

/** Ask the last question again after a failure (drops the error and the question, then resends). */
function retry() {
  const lastUser = state.messages.findLastIndex((m) => m.role === "user");
  if (lastUser < 0 || state.pending) return;
  const question = state.messages[lastUser].content;
  set({ messages: state.messages.slice(0, lastUser) });
  void send(question);
}

/** Record what the user did with a drafted action (confirmed, dismissed, edited). */
function resolveProposal(msgId: string, proposalId: string, patch: Partial<ProposalState>) {
  set({
    messages: state.messages.map((m) => (m.id !== msgId ? m : {
      ...m,
      proposals: m.proposals?.map((p) => (p.id === proposalId ? { ...p, ...patch } : p)),
    })),
  });
}

function clear() {
  stop();
  set({ messages: [] });
}

export const chatActions = {
  send,
  stop,
  retry,
  clear,
  resolveProposal,
  open: () => set({ drawerOpen: true }),
  close: () => set({ drawerOpen: false }),
};

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => listeners.delete(l);
};

/** The shared thread. A different signed-in user than the thread's owner
 *  starts from an empty one, so a shared tab never shows someone else's chat. */
export function useAiChat() {
  const { user } = useAuth();
  const snap = useSyncExternalStore(subscribe, () => state, () => EMPTY);
  useEffect(() => {
    if (user && state.owner !== user.id) {
      stop();
      set({ owner: user.id, messages: [] });
    }
  }, [user]);
  return { ...snap, messages: user && snap.owner === user.id ? snap.messages : [], ...chatActions };
}
