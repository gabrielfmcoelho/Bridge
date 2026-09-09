export default function FormError({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="text-sm rounded-[var(--radius-md)] p-3 animate-slide-down bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)]">
      {message}
    </div>
  );
}
