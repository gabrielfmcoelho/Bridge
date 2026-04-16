interface ListingLabelProps {
  label: string;
  show: boolean;
}

export default function ListingLabel({ label, show }: ListingLabelProps) {
  if (!show) return null;

  return (
    <h2 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
      {label}
    </h2>
  );
}
