export interface ParsedEnv {
  name: string;
  value: string;
  description: string;
}

// parseDotenv turns a pasted .env blob into editable rows. It is deliberately
// permissive: it never rejects a line for a bad name — the caller's row
// validation (^[A-Z_][A-Z0-9_]*$, dup/empty checks) is the single source of
// truth, so a paste shows the same errors a hand-typed row would.
//
// Per line:
//   - blank lines and full-line comments (first char '#') are skipped
//   - a leading `export ` is stripped
//   - split on the FIRST '='; a line without '=' (or empty key) is skipped
//   - inline comment -> description: for an unquoted value, a ' #' (whitespace
//     before '#') starts the comment, so `KEY=pa#ss` keeps `pa#ss` intact
//   - quoted values ("..."/'...') are unquoted and treat '#' as literal; a
//     '#...' after the closing quote becomes the description
//   - key is upper-cased (matches the row editor's onChange), value trimmed
export function parseDotenv(text: string): ParsedEnv[] {
  const out: ParsedEnv[] = [];
  for (const raw of text.split(/\r?\n/)) {
    let line = raw.trim();
    if (line === "" || line.startsWith("#")) continue;
    if (line.startsWith("export ")) line = line.slice(7).trimStart();

    const eq = line.indexOf("=");
    if (eq === -1) continue;
    const name = line.slice(0, eq).trim().toUpperCase();
    if (name === "") continue;

    const rest = line.slice(eq + 1).trimStart();
    let value: string;
    let description = "";

    const quote = rest[0];
    if (quote === '"' || quote === "'") {
      const close = rest.indexOf(quote, 1);
      if (close !== -1) {
        value = rest.slice(1, close);
        const hash = rest.indexOf("#", close + 1);
        if (hash !== -1) description = rest.slice(hash + 1).trim();
      } else {
        // Unterminated quote — keep everything after the opening quote as value.
        value = rest.slice(1).trim();
      }
    } else {
      const m = rest.match(/\s#/);
      if (m && m.index !== undefined) {
        value = rest.slice(0, m.index).trim();
        description = rest.slice(m.index + m[0].length).trim();
      } else {
        value = rest.trim();
      }
    }

    out.push({ name, value, description });
  }
  return out;
}
