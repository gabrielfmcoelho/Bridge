// Scan output carries sizes the way `free -h` / `df -h` print them: "7,2Gi",
// "118G", "512Mi". Shown raw in a small mono face, "G" reads as "6" and the
// Gi/G mix looks like two different units. Spell the unit out instead.
const SIZE = /^\s*(\d+(?:[.,]\d+)?)\s*([KMGTP])(i?)B?\s*$/i;

/** "7,2Gi" → "7,2 GiB", "118G" → "118 GB"; anything else is returned as-is. */
export function formatSize(raw: string): string {
  const m = SIZE.exec(raw);
  if (!m) return raw;
  const [, num, prefix, binary] = m;
  return `${num} ${prefix.toUpperCase()}${binary ? "iB" : "B"}`;
}
