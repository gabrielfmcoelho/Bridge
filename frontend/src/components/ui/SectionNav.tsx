"use client";

import NativeSelect from "./NativeSelect";

export interface SectionNavGroup<K extends string> {
  title: string;
  items: { key: K; label: string }[];
}

/**
 * Vertical, grouped navigation between the sections of one page (settings):
 * the ADS side-nav shape — group titles, one-line items, the active one marked
 * by the accent notch and colour (no fill, like the app sidebar). On phones it
 * collapses to a native select with option groups.
 */
export default function SectionNav<K extends string>({
  label,
  groups,
  value,
  onChange,
}: {
  /** Accessible name of the navigation ("Configurações"). */
  label: string;
  groups: SectionNavGroup<K>[];
  value: K;
  onChange: (key: K) => void;
}) {
  const shown = groups.filter((g) => g.items.length > 0);
  return (
    <>
      <div className="md:hidden">
        <NativeSelect aria-label={label} value={value} onChange={(e) => onChange(e.target.value as K)}>
          {shown.map((g) => (
            <optgroup key={g.title} label={g.title}>
              {g.items.map((i) => (
                <option key={i.key} value={i.key}>{i.label}</option>
              ))}
            </optgroup>
          ))}
        </NativeSelect>
      </div>
      <nav aria-label={label} className="max-md:hidden space-y-4">
        {shown.map((g) => (
          <div key={g.title}>
            <p className="px-3 pb-1.5 text-xs font-semibold text-[var(--text-muted)]">{g.title}</p>
            <ul className="space-y-0.5">
              {g.items.map((i) => {
                const active = i.key === value;
                return (
                  <li key={i.key}>
                    <button
                      type="button"
                      onClick={() => onChange(i.key)}
                      aria-current={active ? "page" : undefined}
                      className={`relative w-full text-left px-3 py-1.5 rounded-[var(--radius-md)] text-sm font-medium truncate transition duration-150 ${
                        active ? "text-[var(--accent)]" : "text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-secondary)]"
                      }`}
                    >
                      {active && <span aria-hidden className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-4 rounded-r-full bg-[var(--accent)]" />}
                      {i.label}
                    </button>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
    </>
  );
}
