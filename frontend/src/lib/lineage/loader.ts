import { Lineage } from "./types";

const LINEAGE_URL = "/lineage/lineage.json";

export async function fetchLineage(signal?: AbortSignal): Promise<Lineage> {
  const res = await fetch(LINEAGE_URL, { signal, cache: "no-store" });
  if (!res.ok) {
    throw new Error(`Failed to load lineage.json (${res.status})`);
  }
  const raw = await res.json();
  // Validate with zod; the parser is the source of truth, so a hard failure
  // here means the upstream schema drifted and needs a backend release.
  return Lineage.parse(raw);
}
