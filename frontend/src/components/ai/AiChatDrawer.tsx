"use client";

import { useState, useRef, useEffect } from "react";
import { aiAPI } from "@/lib/api";
import Drawer from "@/components/ui/Drawer";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

interface Message {
  role: "user" | "assistant";
  content: string;
  error?: boolean;
}

interface AiChatDrawerProps {
  open: boolean;
  onClose: () => void;
}

export default function AiChatDrawer({ open, onClose }: AiChatDrawerProps) {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || loading) return;

    const userMsg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userMsg }]);
    setLoading(true);

    try {
      const res = await aiAPI.chat(userMsg);
      setMessages((prev) => [...prev, { role: "assistant", content: res.response }]);
    } catch (err) {
      const detail = err instanceof Error ? err.message : "Unknown error";
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: `Request failed: ${detail}`, error: true },
      ]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Drawer open={open} onClose={onClose} title="AI Assistant">
      <div className="flex flex-col h-full">
        {/* Messages */}
        <div className="flex-1 overflow-y-auto space-y-3 pb-4">
          {messages.length === 0 && (
            <p className="text-sm text-[var(--text-muted)] text-center py-8">
              Ask me anything about your infrastructure.
            </p>
          )}
          {messages.map((msg, i) => {
            if (msg.role === "user") {
              return (
                <div key={i} className="flex justify-end">
                  <div className="text-sm rounded-[var(--radius-md)] px-3 py-2 max-w-[85%] bg-[var(--accent-muted)] text-[var(--text-primary)]">
                    <div className="whitespace-pre-wrap break-words">{msg.content}</div>
                  </div>
                </div>
              );
            }
            // Assistant messages render as markdown and flow freely — no bubble.
            // Errors keep a subtle red container so they stand out from normal output.
            if (msg.error) {
              return (
                <div
                  key={i}
                  className="text-sm rounded-[var(--radius-md)] px-3 py-2 max-w-[85%] mr-auto bg-[var(--danger)]/10 border border-[var(--danger)]/30 text-[var(--danger)]"
                >
                  <div className="whitespace-pre-wrap break-words">{msg.content}</div>
                </div>
              );
            }
            return (
              <div key={i} className="text-[var(--text-primary)] px-1">
                <MarkdownContent content={msg.content} />
              </div>
            );
          })}
          {loading && (
            <div className="mr-auto bg-[var(--bg-elevated)] text-[var(--text-muted)] rounded-[var(--radius-md)] px-3 py-2 text-sm">
              <span className="animate-pulse">Thinking...</span>
            </div>
          )}
          <div ref={endRef} />
        </div>

        {/* Input */}
        <div className="border-t border-[var(--border-subtle)] pt-3 mt-auto">
          <div className="flex items-end gap-2">
            <textarea
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => {
                // Enter sends; Shift+Enter inserts a newline.
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  handleSend();
                }
              }}
              placeholder="Ask about your infrastructure... (Shift+Enter for newline)"
              rows={3}
              className="flex-1 bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-faint)] focus:outline-none focus:border-[var(--accent)] resize-y min-h-[72px] max-h-[240px] leading-snug"
              disabled={loading}
            />
            <button
              onClick={handleSend}
              disabled={loading || !input.trim()}
              className="shrink-0 h-[40px] px-3 rounded-[var(--radius-md)] bg-[var(--accent)] text-white text-sm font-medium disabled:opacity-40 transition hover:opacity-90 flex items-center justify-center"
              aria-label="Send"
            >
              <Icon path={ICON_PATHS.send} />
            </button>
          </div>
        </div>
      </div>
    </Drawer>
  );
}
