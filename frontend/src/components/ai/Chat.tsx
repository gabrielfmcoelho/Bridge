"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { aiAPI } from "@/lib/api";
import { ICON_PATHS } from "@/lib/icon-paths";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import Button from "@/components/ui/Button";
import CopyButton from "@/components/ui/CopyButton";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import Link from "next/link";
import { citedRefs } from "./chatModel";
import ProposalCard from "./ProposalCard";
import type { ChatMsg } from "./chatStore";

// The assistant's building blocks, shared by the header drawer and the home
// widget. Patterns after shadcn/ui's chat components (MessageScroller,
// Message, Bubble, Marker) and AI Elements' PromptInput/Suggestion, built on
// this repo's own primitives and tokens rather than installed.

/** Messages, newest at the bottom. Follows new messages while the reader is at
 *  the bottom; scrolled up, it stays put and offers a jump back down. */
export function ChatThread({ messages, pending, onRetry, empty, className = "" }: {
  messages: ChatMsg[];
  pending: boolean;
  onRetry: () => void;
  /** Shown while there are no messages (hint, suggestions). */
  empty?: React.ReactNode;
  className?: string;
}) {
  const { t } = useLocale();
  const ref = useRef<HTMLDivElement>(null);
  const [atBottom, setAtBottom] = useState(true);

  useEffect(() => {
    const el = ref.current;
    if (el && atBottom) el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  }, [messages, pending, atBottom]);

  const onScroll = () => {
    const el = ref.current;
    if (el) setAtBottom(el.scrollHeight - el.scrollTop - el.clientHeight < 48);
  };
  const lastId = messages.at(-1)?.id;

  return (
    <div className={`relative min-h-0 ${className}`}>
      <div ref={ref} onScroll={onScroll} role="log" aria-live="polite" aria-busy={pending} className="h-full overflow-y-auto overscroll-contain space-y-4 px-1 py-2">
        {messages.length === 0 && !pending && empty}
        {messages.map((m) =>
          m.role === "user" ? (
            <div key={m.id} className="flex justify-end">
              <p className="max-w-[85%] whitespace-pre-wrap break-words rounded-[var(--radius-lg)] rounded-br-[var(--radius-sm)] bg-[var(--accent)]/10 px-3.5 py-2 text-sm text-[var(--text-primary)]">
                {m.content}
              </p>
            </div>
          ) : m.error ? (
            <div key={m.id} className="flex items-start gap-2 rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 px-3 py-2 text-sm text-[var(--danger)]">
              <Icon path={ICON_PATHS.exclamationCircle} className="mt-0.5 w-4 h-4" />
              <span className="flex-1 break-words">{t("ui.aiChatDrawer.requestFailed", { detail: m.content || t("common.unknownError") })}</span>
              {m.id === lastId && !pending && (
                <Button size="sm" variant="ghost" onClick={onRetry}>{t("ai.chat.retry")}</Button>
              )}
            </div>
          ) : (
            <AssistantTurn key={m.id} msg={m} />
          ),
        )}
        {pending && (
          <p className="flex items-center gap-2 text-xs text-[var(--text-muted)]">
            <span aria-hidden className="h-1.5 w-1.5 rounded-full bg-[var(--accent)] animate-pulse" />
            <span className="animate-pulse">{t("ui.aiChatDrawer.thinking")}</span>
          </p>
        )}
      </div>
      {!atBottom && (
        <IconButton
          label={t("ai.chat.jumpToLatest")}
          onClick={() => ref.current?.scrollTo({ top: ref.current.scrollHeight, behavior: "smooth" })}
          className="absolute bottom-2 left-1/2 -translate-x-1/2 shadow-[var(--elevate-hi)] bg-[var(--bg-surface)]"
        >
          <Icon path={ICON_PATHS.chevronDown} className="w-4 h-4" />
        </IconButton>
      )}
    </div>
  );
}

/** One assistant reply: what it looked up, the answer, the pages it cites, and
 *  any drafted actions waiting for the user. */
function AssistantTurn({ msg: m }: { msg: ChatMsg }) {
  const { t } = useLocale();
  const sources = citedRefs(m.content, m.refs);
  return (
    <div className="group space-y-2">
      {!!m.steps?.length && (
        <ul className="space-y-0.5" aria-label={t("ai.tools.stepsLabel")}>
          {m.steps.map((s, i) => (
            <li key={i} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
              <Icon path={s.error ? ICON_PATHS.exclamationCircle : ICON_PATHS.magnifier} className={`w-3.5 h-3.5 ${s.error ? "text-[var(--warning)]" : ""}`} />
              <span>
                {t(`ai.tools.${s.tool}`)}
                {s.query && <> “{s.query}”</>}
                {s.error ? <> · {t("ai.tools.failed")}</> : !s.tool.startsWith("propose_") && <> · {t("ai.tools.results", { count: String(s.count) })}</>}
              </span>
            </li>
          ))}
        </ul>
      )}
      <MarkdownContent content={m.content} />
      {m.noTools && <p className="text-2xs text-[var(--text-muted)]">{t("ai.tools.unavailable")}</p>}
      {sources.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-2xs text-[var(--text-muted)]">{t("ai.chat.sources")}</span>
          {sources.map((r) => (
            <Link
              key={r.href}
              href={r.href}
              className="rounded-[var(--radius-sm)] border border-[var(--border-subtle)] px-2 py-0.5 text-2xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)]"
            >
              {r.label}
            </Link>
          ))}
        </div>
      )}
      {m.proposals?.map((p) => <ProposalCard key={p.id} msgId={m.id} proposal={p} />)}
      <div className="flex opacity-60 group-hover:opacity-100 focus-within:opacity-100 transition-opacity">
        <CopyButton value={m.content} icon variant="ghost" size="sm" label={t("ai.chat.copy")} />
      </div>
    </div>
  );
}

/** Prompt box: grows with its text, Enter sends, Shift+Enter breaks the line.
 *  Typing stays possible while a reply is pending; the button becomes Stop. */
export function ChatComposer({ pending, onSend, onStop, autoFocus, placeholder }: {
  pending: boolean;
  onSend: (text: string) => void;
  onStop: () => void;
  autoFocus?: boolean;
  placeholder?: string;
}) {
  const { t } = useLocale();
  const [text, setText] = useState("");
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (autoFocus) ref.current?.focus();
  }, [autoFocus]);

  const submit = () => {
    if (pending || !text.trim()) return;
    onSend(text);
    setText("");
  };
  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    // isComposing: Enter that confirms an IME candidate must not send.
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      submit();
    }
  };

  return (
    <div className="flex items-end gap-2 rounded-[var(--radius-lg)] border border-[var(--border-default)] bg-[var(--bg-elevated)] p-1.5 focus-within:border-[var(--accent)] focus-within:ring-2 focus-within:ring-[var(--accent)]/20 transition">
      <textarea
        ref={ref}
        value={text}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={onKeyDown}
        rows={1}
        aria-label={t("ai.chat.inputLabel")}
        placeholder={placeholder ?? t("ui.aiChatDrawer.placeholder")}
        // The wrapper carries the focus ring (border + ring), so no outline here.
        // `!`: the global *:focus-visible rule is unlayered and beats any utility.
        // ponytail: field-sizing grows the box natively (Chromium, Safari 26);
        // elsewhere it stays one line and scrolls. A JS autosize if Firefox users complain.
        className="flex-1 resize-none bg-transparent px-2 py-1.5 text-sm leading-snug text-[var(--text-primary)] placeholder:text-[var(--text-faint)] focus-visible:outline-none! field-sizing-content min-h-9 max-h-40"
      />
      {pending ? (
        <IconButton label={t("ai.chat.stop")} onClick={onStop} variant="default" size="md">
          <Icon path={ICON_PATHS.xCircle} className="w-4 h-4" />
        </IconButton>
      ) : (
        <button
          type="button"
          onClick={submit}
          disabled={!text.trim()}
          aria-label={t("common.send")}
          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)] bg-[var(--accent)] text-white transition hover:opacity-90 active:scale-[0.96] disabled:opacity-40"
        >
          <Icon path={ICON_PATHS.send} className="w-4 h-4" />
        </button>
      )}
    </div>
  );
}

/** Starter prompts for an empty thread; a click sends the prompt as is. */
export function ChatSuggestions({ prompts, onPick }: { prompts: string[]; onPick: (p: string) => void }) {
  return (
    <div className="flex flex-wrap gap-2">
      {prompts.map((p) => (
        <button
          key={p}
          type="button"
          onClick={() => onPick(p)}
          className="rounded-[var(--radius-md)] border border-[var(--border-default)] px-3 py-1.5 text-left text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)] active:scale-[0.98] transition"
        >
          {p}
        </button>
      ))}
    </div>
  );
}

/** Whether the assistant can answer: AI enabled and configured (GET /api/ai/status). */
export function useAiStatus() {
  const q = useQuery({ queryKey: ["ai", "status"], queryFn: aiAPI.status, retry: false, staleTime: 60_000 });
  return {
    loading: q.isLoading,
    ready: !!q.data?.enabled && !!q.data.configured,
    /** Turned off by an admin, as opposed to never set up. */
    disabled: !!q.data && !q.data.enabled && q.data.configured,
  };
}

/** Shown instead of the thread while the assistant can't answer. Admins get
 *  the way to fix it; everyone else learns it isn't their doing. */
export function AiUnavailable({ disabled, compact }: { disabled: boolean; compact?: boolean }) {
  const { t } = useLocale();
  const { user } = useAuth();
  return (
    <div className={`flex items-start gap-3 ${compact ? "" : "py-8"}`}>
      <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-subtle)] text-[var(--text-muted)]">
        <Icon path={ICON_PATHS.sparkles} className="w-4 h-4" />
      </span>
      <div className="min-w-0 space-y-1">
        <p className="text-sm font-medium text-[var(--text-primary)]">{t(disabled ? "ai.unavailable.disabledTitle" : "ai.unavailable.title")}</p>
        <p className="text-sm text-[var(--text-muted)] max-w-[60ch]">
          {t(user?.role === "admin" ? "ai.unavailable.adminHint" : "ai.unavailable.userHint")}
        </p>
        {user?.role === "admin" && (
          <Link href="/settings/integrations" className="inline-flex items-center gap-1 text-xs font-medium text-[var(--accent)] hover:underline">
            {t("ai.unavailable.configure")}
            <Icon path={ICON_PATHS.arrowRight} className="w-3.5 h-3.5" />
          </Link>
        )}
      </div>
    </div>
  );
}
