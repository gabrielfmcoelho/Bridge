"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import RequestList from "./_components/RequestList";

export default function RequestsPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <RequestList />
    </Suspense>
  );
}
