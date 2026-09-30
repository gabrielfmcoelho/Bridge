/**
 * A revealed secret payload as labelled fields, by type: an app login is
 * app / URL / user / password, an SSH key is user / public / private key, a
 * password is just its value. Unknown JSON falls back to its own keys; text
 * that isn't JSON is one "value" field.
 */
export type FieldKind = "text" | "secret" | "url" | "key";
export interface SecretField { key: string; value: string; kind: FieldKind }

const LAYOUT: Record<string, [string, FieldKind][]> = {
  app_login: [["app_name", "text"], ["url", "url"], ["username", "text"], ["password", "secret"], ["notes", "text"]],
  cred: [["username", "text"], ["password", "secret"]],
  sshkey: [["username", "text"], ["public_key", "key"], ["private_key_pem", "key"], ["passphrase", "secret"]],
  password: [["value", "secret"]],
  env_var: [["value", "secret"]],
};

export function secretFields(type: string, raw: string): SecretField[] {
  let obj: Record<string, unknown> | null = null;
  if (raw.trim().startsWith("{")) {
    try {
      const parsed = JSON.parse(raw);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) obj = parsed;
    } catch { /* not JSON: shown as is */ }
  }
  if (!obj) return [{ key: "value", value: raw, kind: "secret" }];
  const str = (v: unknown) => (v == null ? "" : typeof v === "string" ? v : JSON.stringify(v));
  const layout = LAYOUT[type] ?? [];
  const known = new Set(layout.map(([k]) => k));
  const fields: SecretField[] = layout
    .filter(([k]) => str(obj![k]) !== "")
    .map(([k, kind]) => ({ key: k, value: str(obj![k]), kind }));
  // Anything the layout doesn't name still shows, after the known fields.
  for (const [k, v] of Object.entries(obj)) {
    if (!known.has(k) && str(v) !== "") fields.push({ key: k, value: str(v), kind: "text" });
  }
  return fields;
}
