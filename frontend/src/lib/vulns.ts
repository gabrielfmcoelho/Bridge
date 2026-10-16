// Pure helpers for vulnerability findings. Runtime-import free so Node can
// run vulns.test.ts directly.
//
// Finding fields come verbatim from scanner reports and carry live payloads
// (`<script>alert(document.cookie)</script>` in ETIPI evidence): render them
// as React text only, and only ever turn http(s) URLs into links.

/** An http(s) URL as an href; anything else (javascript:, data:, junk) is null. */
export function safeHref(url: string): string | null {
  return /^https?:\/\/[^\s]+$/i.test(url.trim()) ? url.trim() : null;
}

export type TextPart = { text: string; href?: string };

/** Splits free text into plain runs and http(s) URL runs. A URL ends at
 *  whitespace or a " | " separator (ZAP joins several URLs that way). */
export function linkifyParts(text: string): TextPart[] {
  const parts: TextPart[] = [];
  const re = /https?:\/\/[^\s|]+/gi;
  let last = 0;
  for (const m of text.matchAll(re)) {
    const i = m.index ?? 0;
    if (i > last) parts.push({ text: text.slice(last, i) });
    parts.push({ text: m[0], href: m[0] });
    last = i + m[0].length;
  }
  if (last < text.length) parts.push({ text: text.slice(last) });
  return parts;
}

/** "dns:portal.sead.pi.gov.br" → { type: "dns", name: "portal.sead.pi.gov.br" }; "" → null. */
export function parseVia(via: string | undefined): { type: string; name: string } | null {
  if (!via) return null;
  const i = via.indexOf(":");
  return i < 0 ? null : { type: via.slice(0, i), name: via.slice(i + 1) };
}

/** NVD page for a CVE id, or null when it isn't one. */
export function nvdURL(cve: string): string | null {
  return /^CVE-\d{4}-\d{4,}$/.test(cve) ? `https://nvd.nist.gov/vuln/detail/${cve}` : null;
}
