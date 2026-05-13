"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import CatalogPageInner from "@/components/atlas/catalog/CatalogPageInner";

export default function CatalogPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <CatalogPageInner />
    </Suspense>
  );
}
