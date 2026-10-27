import { redirect } from "next/navigation";
import { DEFAULT_SETTINGS_TAB } from "./_tabs";

/** /settings opens its first section; the old /settings?tab=<key> links
 *  land on /settings/<key>. */
export default async function SettingsIndex({ searchParams }: { searchParams: Promise<{ tab?: string | string[] }> }) {
  const { tab } = await searchParams;
  redirect(`/settings/${typeof tab === "string" && /^[a-z-]+$/.test(tab) ? tab : DEFAULT_SETTINGS_TAB}`);
}
