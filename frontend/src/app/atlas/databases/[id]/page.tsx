"use client";

import { use } from "react";
import DatabaseDetail from "./DatabaseDetail";

// Dynamic route params arrive as a Promise, unwrapped with React's `use()`.
export default function DatabaseDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <DatabaseDetail id={Number(id)} />;
}
