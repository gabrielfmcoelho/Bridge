// GLPI's own dropdown picker, for hierarchical itemtypes (ITILCategory,
// Location, ...), shows each ancestor once as a header row followed by its
// children (indented, single "»" prefix regardless of depth) rather than
// repeating the full breadcrumb on every row. AsyncPicker's `path` prop
// reproduces that; this just splits GLPI's completename ("A > B > C") into
// the ancestor list ("A","B") excluding the leaf itself.
export function glpiAncestorPath(completename?: string, name?: string): string[] | undefined {
  if (!completename) return undefined;
  const segments = completename.split(" > ").map((s) => s.trim());
  if (segments.length <= 1) return undefined;
  // Drop the trailing segment — it's the leaf name (== `name`), rendered by
  // the item row itself, not as an ancestor header.
  if (name && segments[segments.length - 1] !== name) {
    // completename's leaf doesn't match name (unexpected shape) — still
    // usable, just keep every segment as context.
    return segments;
  }
  return segments.slice(0, -1);
}
