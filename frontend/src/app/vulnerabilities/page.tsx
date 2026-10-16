"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import VulnList from "./_components/VulnList";

export default function VulnerabilitiesPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <VulnList />
    </Suspense>
  );
}
