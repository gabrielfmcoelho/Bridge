"use client";

import { useState, useRef, useEffect } from "react";
import { aiAPI } from "@/lib/api";
import Drawer from "@/components/ui/Drawer";

interface Message {
  role: "user" | "assistant";
  content: string;
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
    } catch {
      setMessages((prev) => [...prev, { role: "assistant", content: "Sorry, something went wrong." }]);
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
          {messages.map((msg, i) => (
            <div
              key={i}
              className={`text-sm rounded-[var(--radius-md)] px-3 py-2 max-w-[85%] ${
                msg.role === "user"
                  ? "ml-auto bg-[var(--accent-muted)] text-[var(--text-primary)]"
                  : "mr-auto bg-[var(--bg-elevated)] text-[var(--text-secondary)]"
              }`}
            >
              <div className="whitespace-pre-wrap">{msg.content}</div>
            </div>
          ))}
          {loading && (
            <div className="mr-auto bg-[var(--bg-elevated)] text-[var(--text-muted)] rounded-[var(--radius-md)] px-3 py-2 text-sm">
              <span className="animate-pulse">Thinking...</span>
            </div>
          )}
          <div ref={endRef} />
        </div>

        {/* Input */}
        <div className="border-t border-[var(--border-subtle)] pt-3 mt-auto">
          <div className="flex gap-2">
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && handleSend()}
              placeholder="Ask about your infrastructure..."
              className="flex-1 bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)] placeholder:text-[var(--text-faint)] focus:outline-none focus:border-[var(--accent)]"
              disabled={loading}
            />
            <button
              onClick={handleSend}
              disabled={loading || !input.trim()}
              className="px-3 py-2 rounded-[var(--radius-md)] bg-[var(--accent)] text-white text-sm font-medium disabled:opacity-40 transition-all hover:opacity-90"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 12L3.269 3.126A59.768 59.768 0 0121.485 12 59.77 59.77 0 013.27 20.876L5.999 12zm0 0h7.5" />
              </svg>
            </button>
          </div>
        </div>
      </div>
    </Drawer>
  );
}
