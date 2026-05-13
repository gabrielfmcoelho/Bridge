"use client";

import { Suspense } from "react";
import PageShell from "@/components/layout/PageShell";
import { Skeleton } from "@/components/ui/Skeleton";
import PipelinePageInner from "@/components/atlas/pipeline/PipelinePageInner";

export default function PipelinePage() {
  return (
    <Suspense
      fallback={
        <PageShell fullBleed>
          <Skeleton className="w-full h-[60vh] rounded-[var(--radius-lg)]" />
        </PageShell>
      }
    >
      <PipelinePageInner />
    </Suspense>
  );
}
