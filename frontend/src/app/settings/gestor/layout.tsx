"use client";

import type { ReactNode } from "react";
import { notFound } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

/** /settings/gestor/*: admin-only, like the Settings tabs. */
export default function GestorSettingsLayout({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  if (!user) return null; // still signing in
  if (user.role !== "admin") notFound();
  return <>{children}</>;
}
