export interface MetadataItem {
  label: string;
  value: string;
  mono?: boolean;
}

/** Label+value metadata grid for inventory cards. Pairs with no value are not
 *  rendered at all: a column of "-" is noise, and letting the card collapse to
 *  its real height makes height itself readable when scanning a grid. */
export default function CardMetadataGrid({ items }: { items: MetadataItem[] }) {
  const shown = items.filter((i) => i.value && i.value.trim() && i.value.trim() !== "-");
  if (!shown.length) return null;

  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-3">
      {shown.map((item) => (
        <div key={item.label}>
          <span className="text-xs text-[var(--text-muted)]">{item.label}</span>
          <p className="text-xs text-[var(--text-secondary)] truncate font-mono">{item.value}</p>
        </div>
      ))}
    </div>
  );
}
