export interface MetadataItem {
  label: string;
  value: string;
  mono?: boolean;
}

/** 2x2 label+value metadata grid for inventory cards. */
export default function CardMetadataGrid({ items }: { items: MetadataItem[] }) {
  return (
    <div className="grid grid-cols-2 gap-x-4 gap-y-3 min-h-[60px]">
      {items.map((item) => (
        <div key={item.label}>
          <span className="text-xs text-[var(--text-faint)]">{item.label}</span>
          <p
            className="text-xs text-[var(--text-secondary)] truncate"
            style={item.mono ? { fontFamily: "var(--font-mono)" } : undefined}
          >
            {item.value || "-"}
          </p>
        </div>
      ))}
    </div>
  );
}
