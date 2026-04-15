export default function FormError({ message }: { message: string }) {
  if (!message) return null;
  return (
    <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3 animate-slide-down">
      {message}
    </div>
  );
}
