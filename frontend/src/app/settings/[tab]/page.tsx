"use client";

import { notFound, useParams } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { SETTINGS_TABS, canSeeTab } from "../_tabs";

/** /settings/<key>: one Settings section. An unknown key, or one this role
 *  can't see, is a 404 — the nav never offers it either. */
export default function SettingsTabPage() {
  const { tab } = useParams<{ tab: string }>();
  const { user } = useAuth();
  const entry = SETTINGS_TABS.find((s) => s.key === tab);
  if (!entry) notFound();
  if (!user) return null; // still signing in
  if (!canSeeTab(entry, user.role)) notFound();
  return <entry.Component />;
}
