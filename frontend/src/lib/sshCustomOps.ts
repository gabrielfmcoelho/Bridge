// {{name}} placeholders of a custom SSH operation's script, in first-use
// order, deduplicated. Twin of customOpPlaceholder in
// internal/api/ssh_custom_ops.go — keep the two patterns identical.
export function scriptParams(script: string): string[] {
  return [...new Set(Array.from(script.matchAll(/\{\{([a-z_][a-z0-9_]*)\}\}/g), (m) => m[1]))];
}
