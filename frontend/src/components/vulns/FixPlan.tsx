"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import SectionHeading from "@/components/ui/SectionHeading";
import Button from "@/components/ui/Button";
import FormError from "@/components/ui/FormError";
import { Skeleton } from "@/components/ui/Skeleton";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { vulnAPI } from "@/lib/api";
import { getTimeAgo } from "@/lib/utils";

export const fixPlanKey = (findingId: number) => ["vulnerabilities", "fix-plan", findingId];

/** The finding's case-specific fix plan: the LLM explored the linked repos
 *  and host scans, then wrote what to change here. Shows what it read. */
export default function FixPlan({ findingId }: { findingId: number }) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const canGenerate = !!user?.permissions?.includes("ai.use");
  const queryClient = useQueryClient();
  const [error, setError] = useState("");

  const { data: plan, isLoading } = useQuery({ queryKey: fixPlanKey(findingId), queryFn: () => vulnAPI.fixPlan(findingId) });
  const generate = useMutation({
    mutationFn: () => vulnAPI.generateFixPlan(findingId),
    onSuccess: (fresh) => {
      setError("");
      queryClient.setQueryData(fixPlanKey(findingId), fresh);
    },
    onError: (e: Error) => setError(e.message),
  });

  return (
    <section className="space-y-2">
      <SectionHeading
        className="!mb-0"
        actions={
          canGenerate && (
            <Button size="sm" variant="secondary" loading={generate.isPending} onClick={() => generate.mutate()}>
              {plan ? t("vuln.fixPlan.regenerate") : t("vuln.fixPlan.generate")}
            </Button>
          )
        }
      >
        {t("vuln.fixPlan.title")}
      </SectionHeading>
      <p className="text-xs text-[var(--text-muted)]">{t("vuln.fixPlan.hint")}</p>

      <FormError message={error} />
      {generate.isPending && <p className="text-xs text-[var(--text-muted)]">{t("vuln.fixPlan.running")}</p>}

      {isLoading ? (
        <Skeleton className="h-24 w-full" />
      ) : plan ? (
        <>
          <MarkdownContent content={plan.content} />
          <details className="text-xs text-[var(--text-muted)]">
            <summary className="cursor-pointer select-none">{t("vuln.fixPlan.howMade", { steps: String(plan.steps.length) })}</summary>
            <div className="mt-2 space-y-2">
              <p>
                {t("vuln.fixPlan.repos")}: {plan.repos.length ? <span className="font-mono">{plan.repos.join(", ")}</span> : t("vuln.fixPlan.noRepos")}
              </p>
              {plan.steps.length > 0 && (
                <ol className="list-decimal pl-5 space-y-0.5 font-mono">
                  {plan.steps.map((s, i) => (
                    <li key={i} className={s.error ? "text-[var(--danger)]" : undefined}>
                      {s.tool} {s.query}
                      {s.error ? ` — ${s.error}` : ""}
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </details>
          <p className="text-2xs text-[var(--text-muted)]">
            {t("vuln.fixPlan.meta", { ago: getTimeAgo(plan.generated_at, locale), model: plan.model || "—" })}
          </p>
        </>
      ) : (
        !generate.isPending && (
          <p className="text-sm text-[var(--text-muted)]">{canGenerate ? t("vuln.fixPlan.empty") : t("vuln.fixPlan.emptyReadOnly")}</p>
        )
      )}
    </section>
  );
}
