import type { Contact } from "./types";

export function contactsToOptions(contacts: Contact[]): { value: string; label: string }[] {
  return contacts
    .filter((c) => c.name)
    .reduce((acc, c) => {
      if (!acc.some((o) => o.value === c.name))
        acc.push({ value: c.name, label: c.name + (c.phone ? ` (${c.phone})` : "") });
      return acc;
    }, [] as { value: string; label: string }[]);
}

/** Parse a percentage string like "45%" to a number. */
export function parsePercent(s?: string): number {
  return parseInt((s || "0").replace("%", "")) || 0;
}

/** Resolve which SSH auth method to use. */
export function resolveAuthMethod(
  hasPassword: boolean,
  hasKey: boolean,
  preferredAuth?: "password" | "key" | ""
): "password" | "key" | null {
  if (hasPassword && hasKey) {
    if (preferredAuth === "password" || preferredAuth === "key") return preferredAuth;
    return null;
  }
  if (hasPassword) return "password";
  if (hasKey) return "key";
  return null;
}

/** Get a CSS color class for a percentage value (red >= 80, amber >= 50). */
export function pctTextColor(pct: number): string {
  return pct >= 80 ? "text-red-400" : pct >= 50 ? "text-amber-400" : "text-[var(--text-secondary)]";
}

/** Get a CSS background class for a percentage bar. */
export function pctBarColor(pct: number): string {
  return pct >= 80 ? "bg-red-500" : pct >= 50 ? "bg-amber-500" : "bg-emerald-500";
}

/** Format a relative time ago string from an ISO date. Pass locale="pt-BR" for Portuguese. */
export function getTimeAgo(dateStr: string, locale: string = "en"): string {
  const date = new Date(dateStr);
  const diffMs = Date.now() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);

  if (locale === "pt-BR") {
    if (diffMin < 1) return "agora";
    if (diffMin < 60) return `há ${diffMin} ${diffMin === 1 ? "minuto" : "minutos"}`;
    const diffH = Math.floor(diffMin / 60);
    if (diffH < 24) return `há ${diffH} ${diffH === 1 ? "hora" : "horas"}`;
    const diffD = Math.floor(diffH / 24);
    if (diffD < 30) return `há ${diffD} ${diffD === 1 ? "dia" : "dias"}`;
    return date.toLocaleDateString("pt-BR");
  }

  if (diffMin < 1) return "just now";
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH}h ago`;
  const diffD = Math.floor(diffH / 24);
  if (diffD < 30) return `${diffD}d ago`;
  return date.toLocaleDateString();
}

/** Check if scan output contains permission/error messages. */
export function hasPermissionDeniedMessage(text?: string): boolean {
  if (!text) return false;
  const lower = text.toLowerCase();
  return lower.includes("permission") || lower.includes("denied") || lower.includes("bash") || lower.includes("/dev/null") || lower.includes("error");
}

/** Format raw uptime string into compact abbreviation. */
export function formatUptime(raw: string, locale: string): string {
  const s = raw.replace(/^up\s+/i, "").trim();
  const abbr = locale === "pt-BR"
    ? { y: "a", w: "s", d: "d", h: "h", m: "m" }
    : { y: "y", w: "w", d: "d", h: "h", m: "m" };
  const parts: string[] = [];
  const yMatch = s.match(/(\d+)\s*year/); if (yMatch) parts.push(`${yMatch[1]}${abbr.y}`);
  const wMatch = s.match(/(\d+)\s*week/); if (wMatch) parts.push(`${wMatch[1]}${abbr.w}`);
  const dMatch = s.match(/(\d+)\s*day/); if (dMatch) parts.push(`${dMatch[1]}${abbr.d}`);
  const hMatch = s.match(/(\d+)\s*hour/); if (hMatch) parts.push(`${hMatch[1]}${abbr.h}`);
  const mMatch = s.match(/(\d+)\s*min/); if (mMatch) parts.push(`${mMatch[1]}${abbr.m}`);
  return parts.length > 0 ? parts.join(" ") : s;
}

/** Parse a `last` command login entry. */
export function parseLoginEntry(raw: string): { user: string; from: string; when: string } {
  const parts = raw.trim().split(/\s+/);
  return { user: parts[0] || "", from: parts[2] || "", when: parts.slice(3).join(" ") };
}

/** Format a login date string to locale-aware short format. */
export function formatLoginDate(when: string, locale: string): string {
  const match = when.match(/^(\w+)\s+(\w+)\s+(\d+)\s+([\d:]+)/);
  if (!match) return when;
  const [, , mon, day, time] = match;
  const months: Record<string, number> = { Jan: 0, Feb: 1, Mar: 2, Apr: 3, May: 4, Jun: 5, Jul: 6, Aug: 7, Sep: 8, Oct: 9, Nov: 10, Dec: 11 };
  const monthNum = months[mon];
  if (monthNum === undefined) return when;
  const now = new Date();
  const d = new Date(now.getFullYear(), monthNum, parseInt(day));
  if (d > now) d.setFullYear(d.getFullYear() - 1);
  const bcp47 = locale === "pt-BR" ? "pt-BR" : "en-US";
  const datePart = d.toLocaleDateString(bcp47, { day: "2-digit", month: "2-digit", year: "2-digit" });
  const durMatch = when.match(/\(([^)]+)\)/);
  return `${datePart} ${time}${durMatch ? ` (${durMatch[1]})` : ""}`;
}

/** Port/service icon lookup. */
const PORT_ICONS: Record<string, string> = {
  "3306": "\uD83D\uDC2C", "mysql": "\uD83D\uDC2C", "mariadb": "\uD83D\uDC2C",
  "5432": "\uD83D\uDC18", "postgres": "\uD83D\uDC18",
  "27017": "\uD83C\uDF43", "mongo": "\uD83C\uDF43",
  "6379": "\uD83D\uDD34", "redis": "\uD83D\uDD34",
  "80": "\uD83C\uDF10", "443": "\uD83D\uDD12", "8080": "\uD83C\uDF10", "8443": "\uD83D\uDD12",
  "nginx": "\uD83C\uDF10", "apache": "\uD83C\uDF10", "httpd": "\uD83C\uDF10", "caddy": "\uD83C\uDF10",
  "3000": "\u26A1", "5000": "\uD83D\uDC0D", "8000": "\uD83D\uDC0D",
  "node": "\u26A1", "python": "\uD83D\uDC0D", "java": "\u2615", "gunicorn": "\uD83D\uDC0D", "uvicorn": "\uD83D\uDC0D",
  "php-fpm": "\uD83D\uDC18",
  "22": "\uD83D\uDD11", "sshd": "\uD83D\uDD11",
  "53": "\uD83D\uDCE1", "9090": "\uD83D\uDCCA", "9100": "\uD83D\uDCCA",
  "docker": "\uD83D\uDC33", "containerd": "\uD83D\uDC33",
  "traefik": "\uD83D\uDD00", "haproxy": "\uD83D\uDD00",
};

export function portIcon(text: string): string {
  const lower = text.toLowerCase();
  for (const [key, icon] of Object.entries(PORT_ICONS)) {
    if (lower.includes(key)) return icon;
  }
  return "\u00B7";
}

/** Parse a service_details entry. */
export function parseServiceRow(raw: string) {
  const match = raw.match(/^(.+?)\s+CPU:([\d.]+%)\s+MEM:([\d.]+%)\s+RSS:(.+)$/);
  const name = match?.[1] || raw;
  const cpu = match?.[2] || "0%";
  const mem = match?.[3] || "0%";
  const rss = (match?.[4] || "0").trim();
  return { name, cpu, mem, rss, cpuNum: parseFloat(cpu) || 0, memNum: parseFloat(mem) || 0, rssNum: parseFloat(rss) || 0 };
}

/** Like pctTextColor but uses emerald instead of text-secondary for low values (compact indicators). */
export function pctTextColorVivid(pct: number): string {
  return pct >= 80 ? "text-red-400" : pct >= 50 ? "text-amber-400" : "text-emerald-400";
}

/** Check if scan_resources contain any permission-denied-like messages. */
export function hasScanPermissionWarning(sr: {
  cpu?: string; cpu_usage?: string; ram?: string; ram_percent?: string; storage?: string; disk_percent?: string;
}): boolean {
  return hasPermissionDeniedMessage(sr.cpu)
    || hasPermissionDeniedMessage(sr.cpu_usage)
    || hasPermissionDeniedMessage(sr.ram)
    || hasPermissionDeniedMessage(sr.ram_percent)
    || hasPermissionDeniedMessage(sr.storage)
    || hasPermissionDeniedMessage(sr.disk_percent);
}

/** Parse a container_stats entry. */
export function parseContainerRow(raw: string) {
  const nameMatch = raw.match(/^(.+?)\s+CPU:/);
  const name = nameMatch?.[1]?.trim() || raw.split(/\s+/)[0] || raw;
  const cpuMatch = raw.match(/CPU:([\d.]+%?)/);
  const cpu = cpuMatch?.[1] || "";
  const memMatch = raw.match(/MEM:(.+?)(?:\s+NET:|$)/);
  const mem = memMatch?.[1]?.trim() || "";
  const netMatch = raw.match(/NET:(.+)$/);
  const net = netMatch?.[1]?.trim() || "";
  return { name, cpu, mem, net, cpuNum: parseFloat(cpu) || 0, memNum: parseFloat(mem) || 0 };
}
