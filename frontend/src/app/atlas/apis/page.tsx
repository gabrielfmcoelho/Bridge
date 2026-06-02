"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import ApiListView from "@/components/atlas/apis/ApiListView";

export default function ApisPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <ApiListView />
    </Suspense>
  );
}
