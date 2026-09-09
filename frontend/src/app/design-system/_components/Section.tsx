import type { ReactNode } from "react";

// One anchor-addressable block of the catalog. Ids are fixed by SECTIONS in page.tsx.
export function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    // [&_code]:break-all — brace-grouped paths like app/{a,b}/x.tsx have no break opportunity and overflow phones.
    <section id={id} className="scroll-mt-20 space-y-6 [&_code]:break-all">
      <h2 className="text-lg font-semibold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
        {title}
      </h2>
      {children}
    </section>
  );
}

// One component (or ad-hoc specimen) with the path that defines it and the paths that duplicate it.
export function Specimen({
  title,
  source,
  alsoIn,
  wide = false,
  children,
}: {
  title: string;
  source: string;
  alsoIn?: string[];
  wide?: boolean;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">{title}</h3>
        <code className="text-[11px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
          {source}
        </code>
      </div>
      <div
        className={`rounded-[var(--radius-lg)] border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-4 ${
          wide ? "" : "flex flex-wrap items-start gap-3"
        }`}
      >
        {children}
      </div>
      {alsoIn && alsoIn.length > 0 && (
        <p className="text-[11px] text-[var(--text-muted)]">
          Also implemented in:{" "}
          <code style={{ fontFamily: "var(--font-mono)" }}>{alsoIn.join(", ")}</code>
        </p>
      )}
    </div>
  );
}
