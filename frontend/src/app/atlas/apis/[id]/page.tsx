"use client";

import { Suspense, use } from "react";
import ApiDetail from "./ApiDetail";

// Dynamic route params arrive as a Promise, unwrapped with React's `use()`.
// The Suspense boundary is required because ApiDetail reads useSearchParams
// (the ?op deep-jump param).
export default function ApiDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ApiDetail id={Number(id)} />
    </Suspense>
  );
}
