/**
 * Renders the standard monospace output block used across SSH operation results.
 * Displays message and/or error with optional raw output appended.
 */
export default function OperationOutput({ data }: {
  data: { message?: string; error?: string; output?: string };
}) {
  const text = data.message || data.error || "";
  const suffix = data.output ? "\n" + data.output : "";
  return (
    <pre
      className="whitespace-pre-wrap text-[10px]"
      style={{ fontFamily: "var(--font-mono)" }}
    >
      {text}{suffix}
    </pre>
  );
}
