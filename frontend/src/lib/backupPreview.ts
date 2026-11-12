// What a host backup would take, from the host's latest scan — before it
// runs. Pure (types only), so `node --test` runs it.
import type { VMInfoType } from "./api";

export type FSState = "included" | "partial" | "excluded" | "network" | "outside";

export interface PreviewFS {
  mount: string;
  fstype: string;
  used: number;
  /** included: the whole mount; partial: only some paths under it. */
  state: FSState;
}

/** A one-click change to the paths, with why. */
export interface Suggestion {
  kind: "database" | "docker" | "bigfile";
  /** Database label, file path, or container count. */
  subject: string;
  bytes?: number;
  /** Paths the exclude list gains / loses. */
  addExclude: string[];
  removeExclude: string[];
}

export interface BackupPreview {
  filesystems: PreviewFS[];
  /** Used bytes of what the include covers: an upper bound (excluded
   *  folders inside a filesystem aren't subtracted, compression isn't counted). */
  upperBound: number;
  suggestions: Suggestion[];
}

/** path is root or below it. */
export function under(path: string, root: string): boolean {
  if (root === "/") return path.startsWith("/");
  const r = root.replace(/\/+$/, "");
  return path === r || path.startsWith(r + "/");
}

/** Whether a backup with these lists takes path (an exclude wins over an
 *  include beneath it, like GNU tar). */
export function covered(path: string, include: string[], exclude: string[]): boolean {
  return include.some((i) => under(path, i)) && !exclude.some((e) => under(path, e));
}

/** Default data folders of databases running on the host. */
export const DB_DATA_DIRS: [string, string][] = [
  ["postgres", "/var/lib/postgresql"],
  ["mysql", "/var/lib/mysql"],
  ["mariadb", "/var/lib/mysql"],
  ["mongo", "/var/lib/mongodb"],
  ["redis", "/var/lib/redis"],
  ["elasticsearch", "/var/lib/elasticsearch"],
  ["influx", "/var/lib/influxdb"],
];

export const DOCKER_ROOT = "/var/lib/docker";
/** Docker's folders other than volumes: excluding these instead of the root
 *  keeps the volumes (an exclude of the root would drop them, includes or not). */
export const DOCKER_NON_VOLUMES = ["overlay2", "image", "containers", "buildkit", "tmp"].map((d) => `${DOCKER_ROOT}/${d}`);

/** Files at least this big get an "exclude it?" suggestion. */
export const BIG_FILE = 1024 ** 3;

export function backupPreview(info: Partial<VMInfoType> | undefined, include: string[], exclude: string[]): BackupPreview {
  const filesystems: PreviewFS[] = (info?.filesystems ?? []).map((fs) => {
    let state: FSState = "outside";
    if (fs.network) state = "network";
    else if (exclude.some((e) => under(fs.mount, e))) state = "excluded";
    else if (include.some((i) => under(fs.mount, i))) state = "included";
    else if (include.some((i) => under(i, fs.mount))) state = "partial";
    return { mount: fs.mount, fstype: fs.fstype, used: fs.used_bytes, state };
  });
  // A path belongs to its deepest mount; a "partial" mount whose included
  // paths sit on a deeper mount isn't counted twice in practice: rare enough.
  const upperBound = filesystems.filter((f) => f.state === "included" || f.state === "partial").reduce((n, f) => n + f.used, 0);

  const suggestions: Suggestion[] = [];
  const seen = new Set<string>();
  for (const s of info?.service_inventory ?? []) {
    if (s.kind !== "database" || !s.host_running) continue;
    const dir = DB_DATA_DIRS.find(([k]) => s.name.toLowerCase().includes(k))?.[1];
    if (!dir || seen.has(dir) || !covered(dir, include, exclude)) continue;
    seen.add(dir);
    suggestions.push({ kind: "database", subject: s.label || s.name, addExclude: [dir], removeExclude: [] });
  }
  const containers = info?.parsed_containers?.length ?? 0;
  if (containers > 0 && exclude.includes(DOCKER_ROOT) && include.some((i) => under(DOCKER_ROOT, i))) {
    suggestions.push({ kind: "docker", subject: String(containers), addExclude: DOCKER_NON_VOLUMES, removeExclude: [DOCKER_ROOT] });
  }
  for (const f of info?.resource_top?.top_disk ?? []) {
    if (f.size_bytes >= BIG_FILE && covered(f.path, include, exclude)) {
      suggestions.push({ kind: "bigfile", subject: f.path, bytes: f.size_bytes, addExclude: [f.path], removeExclude: [] });
    }
  }
  return { filesystems, upperBound, suggestions };
}

/** The exclude list after taking a suggestion (no duplicates, order kept). */
export function applySuggestion(exclude: string[], s: Suggestion): string[] {
  const out = exclude.filter((e) => !s.removeExclude.includes(e));
  for (const e of s.addExclude) if (!out.includes(e)) out.push(e);
  return out;
}

/** "a b\nc" → ["a", "b", "c"]. */
export const parsePaths = (text: string): string[] => text.split(/\s+/).filter(Boolean);

/** 3400000000 → "3.2 GiB". */
export function formatBytes(n: number): string {
  if (n <= 0) return "0";
  if (n >= 1024 ** 4) return `${(n / 1024 ** 4).toFixed(1)} TiB`;
  if (n >= 1024 ** 3) return `${(n / 1024 ** 3).toFixed(1)} GiB`;
  return `${Math.max(1, Math.round(n / 1024 ** 2))} MiB`;
}
