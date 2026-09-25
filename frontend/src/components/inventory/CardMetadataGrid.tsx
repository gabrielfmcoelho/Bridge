export interface MetadataItem {
  label: string;
  value: string;
  mono?: boolean;
}

/** Label+value metadata grid for inventory cards. Fixed anatomy: every pair
 *  always renders in the same cell, an empty value as a muted "–", so the
 *  admin learns where each fact lives and cards in a row line up. */
export default function CardMetadataGrid({ items }: { items: MetadataItem[] }) {
  const empty = (v: string) => !v || !v.trim() || v.trim() === "-";

  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-3">
      {items.map((item) => (
        <div key={item.label}>
          <span className="text-xs text-[var(--text-muted)]">{item.label}</span>
          {empty(item.value) ? (
            <p className="text-xs text-[var(--text-muted)]">–</p>
          ) : (
            <p className={`text-xs text-[var(--text-secondary)] truncate ${item.mono ? "font-mono" : ""}`} title={item.value}>{item.value}</p>
          )}
        </div>
      ))}
    </div>
  );
}
