"use client";

import { useState } from "react";

export interface AccordionSection {
  id: string;
  title: string;
  content: React.ReactNode;
}

interface AccordionProps {
  sections: AccordionSection[];
  defaultOpen?: string;
}

export default function Accordion({ sections, defaultOpen }: AccordionProps) {
  const [openId, setOpenId] = useState<string | null>(defaultOpen ?? sections[0]?.id ?? null);

  const toggle = (id: string) => {
    setOpenId((prev) => (prev === id ? null : id));
  };

  return (
    <div className="space-y-2">
      {sections.map((section) => {
        const isOpen = openId === section.id;
        return (
          <div
            key={section.id}
            className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden transition-colors"
          >
            <button
              type="button"
              onClick={() => toggle(section.id)}
              className="w-full flex items-center justify-between px-4 py-2.5 bg-[var(--bg-elevated)] text-sm font-medium text-[var(--text-secondary)] hover:bg-[var(--bg-overlay)] transition-colors"
              style={{ fontFamily: "var(--font-display)" }}
            >
              {section.title}
              <svg
                className={`w-4 h-4 transition-transform duration-200 ${isOpen ? "rotate-180" : ""}`}
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={2}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
              </svg>
            </button>
            <div
              className={`grid transition-all duration-200 ${isOpen ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
            >
              <div className="overflow-hidden">
                <div className="p-4 space-y-3">{section.content}</div>
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
