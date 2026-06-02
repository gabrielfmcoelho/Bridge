"use client";

import { Suspense, use } from "react";
import ApiDetailInner from "@/components/atlas/apis/ApiDetailInner";

// In this Next.js, dynamic route params arrive as a Promise and are unwrapped
// with React's `use()` — same pattern as app/share/[token]/page.tsx. The
// Suspense boundary is required because ApiDetailInner reads useSearchParams
// (the ?op deep-jump param).
export default function ApiDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <Suspense fallback={null}>
      <ApiDetailInner id={Number(id)} />
    </Suspense>
  );
}
