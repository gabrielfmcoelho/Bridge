import { useTheme } from "@/contexts/ThemeContext";

export default function FormError({ message }: { message: string }) {
  const { theme } = useTheme();
  if (!message) return null;
  return (
    <div className={`text-sm rounded-[var(--radius-md)] p-3 animate-slide-down ${
      theme === "light"
        ? "bg-red-50 border border-red-200 text-red-700"
        : "bg-red-500/10 border border-red-500/25 text-red-400"
    }`}>
      {message}
    </div>
  );
}
