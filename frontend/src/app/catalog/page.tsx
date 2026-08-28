"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import CatalogSearch from "./_components/CatalogSearch";

export default function CatalogPage() {
  return (
    <Suspense
      fallback={
        <PageShell>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <CatalogSearch />
    </Suspense>
  );
}
