"use client";

import { use } from "react";
import AppDepDetail from "./AppDepDetail";

// Dynamic route params arrive as a Promise, unwrapped with React's `use()`.
export default function AppDepDetailPage({ params }: { params: Promise<{ ci: string }> }) {
  const { ci } = use(params);
  return <AppDepDetail ci={decodeURIComponent(ci)} />;
}
