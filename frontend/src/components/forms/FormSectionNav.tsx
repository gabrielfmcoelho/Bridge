"use client";

import { useEffect, useRef, useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import PillButton from "@/components/ui/PillButton";
import type { FormSectionRef } from "./EntityFormShell";

/** The form's section index (drawer sub-header): one pill per section,
 *  scrolls to it, highlights the one in view. */
export default function FormSectionNav({ sections }: { sections: FormSectionRef[] }) {
  const { t } = useLocale();
  const [current, setCurrent] = useState(sections[0]?.id);
  // A click wins over the observer while its smooth scroll is still moving
  // (the last sections never reach the top band, so the observer would pick
  // an earlier one).
  const pinnedUntil = useRef(0);

  useEffect(() => {
    const els = sections.map((s) => document.getElementById(s.id)).filter((el): el is HTMLElement => !!el);
    if (!els.length) return;
    const io = new IntersectionObserver(
      (entries) => {
        if (Date.now() < pinnedUntil.current) return;
        const top = entries.filter((e) => e.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)[0];
        if (top) setCurrent(top.target.id);
      },
      // The band just under the sub-header decides which section is current.
      { rootMargin: "0px 0px -70% 0px" },
    );
    els.forEach((el) => io.observe(el));
    return () => io.disconnect();
  }, [sections]);

  return (
    <nav aria-label={t("form.sectionsLabel")} className="flex gap-1.5 overflow-x-auto -mx-1 px-1 pb-0.5 [scrollbar-width:none]">
      {sections.map((s) => (
        <PillButton
          key={s.id}
          shape="pill"
          size="sm"
          active={current === s.id}
          aria-current={current === s.id ? "true" : undefined}
          onClick={() => {
            pinnedUntil.current = Date.now() + 900;
            setCurrent(s.id);
            document.getElementById(s.id)?.scrollIntoView({ behavior: "smooth", block: "start" });
          }}
          className="shrink-0"
        >
          {s.label}
        </PillButton>
      ))}
    </nav>
  );
}
