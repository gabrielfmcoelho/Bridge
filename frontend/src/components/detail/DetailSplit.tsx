import type { ReactNode } from "react";

/**
 * Detail "Visão geral": what was declared (the profile, sticky) beside what
 * was observed (scans, runtime, certificate), which scrolls. Stacks below lg.
 * Host detail is the reference (DESIGN_SYSTEM.md rule 3, "Detail pages").
 */
export default function DetailSplit({ profile, children }: { profile: ReactNode; children: ReactNode }) {
  return (
    <div className="lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-6 max-lg:space-y-6 animate-fade-in">
      <aside className="lg:sticky lg:top-0 lg:self-start lg:max-h-[calc(100vh-5.5rem)] lg:overflow-y-auto space-y-5">
        {profile}
      </aside>
      <div className="min-w-0 space-y-5">{children}</div>
    </div>
  );
}
