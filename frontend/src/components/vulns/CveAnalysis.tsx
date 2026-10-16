"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import SectionHeading from "@/components/ui/SectionHeading";
import PillButton from "@/components/ui/PillButton";
import Button from "@/components/ui/Button";
import FormError from "@/components/ui/FormError";
import Field from "@/components/ui/Field";
import { Skeleton } from "@/components/ui/Skeleton";
import { MarkdownContent } from "@/components/ui/MarkdownEditor";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { vulnAPI } from "@/lib/api";
import { getTimeAgo } from "@/lib/utils";
import { nvdURL, safeHref } from "@/lib/vulns";

type Lang = "pt" | "en";

/** The CVE catalog entry for one CVE: the generic pt-BR / en analysis, and
 *  with findingId the "Analisar" button (it runs through that finding, so the
 *  catalog drawer, which has none, is read-only). showRecord adds the NVD
 *  text itself, for the catalog where no finding shows it. */
export default function CveAnalysis({ cve, findingId, showRecord }: { cve: string; findingId?: number; showRecord?: boolean }) {
  const { t, locale } = useLocale();
  const { user } = useAuth();
  const canAnalyze = findingId != null && !!user?.permissions?.includes("ai.use");
  const queryClient = useQueryClient();
  const [lang, setLang] = useState<Lang>(locale === "en" ? "en" : "pt");
  const [error, setError] = useState("");

  const key = ["vulnerabilities", "cve", cve];
  const { data: entry, isLoading } = useQuery({ queryKey: key, queryFn: () => vulnAPI.cve(cve) });
  const analyze = useMutation({
    mutationFn: () => vulnAPI.analyzeCve(findingId!),
    onSuccess: (fresh) => {
      setError("");
      queryClient.setQueryData(key, fresh);
      queryClient.invalidateQueries({ queryKey: ["vulnerabilities", "cves"] });
    },
    onError: (e: Error) => setError(e.message),
  });

  const text = entry ? (lang === "pt" ? entry.analysis_pt : entry.analysis_en) : "";
  const nvd = nvdURL(cve);

  return (
    <section className="space-y-2">
      <SectionHeading
        className="!mb-0"
        actions={
          <div className="flex items-center gap-1.5">
            {entry && (
              <>
                <PillButton size="sm" active={lang === "pt"} onClick={() => setLang("pt")}>pt-BR</PillButton>
                <PillButton size="sm" active={lang === "en"} onClick={() => setLang("en")}>en</PillButton>
              </>
            )}
            {canAnalyze && (
              <Button size="sm" variant="secondary" loading={analyze.isPending} onClick={() => analyze.mutate()}>
                {entry ? t("vuln.cveAnalysis.reanalyze") : t("vuln.cveAnalysis.analyze")}
              </Button>
            )}
          </div>
        }
      >
        {t("vuln.cveAnalysis.title")}
      </SectionHeading>
      <p className="text-xs text-[var(--text-muted)]">{t("vuln.cveAnalysis.hint")}</p>

      <FormError message={error} />
      {analyze.isPending && <p className="text-xs text-[var(--text-muted)]">{t("vuln.cveAnalysis.running")}</p>}

      {showRecord && entry && (
        <div className="space-y-3">
          <div className="grid grid-cols-2 gap-3">
            <Field label={t("vuln.cve")} value={entry.cve_id} mono href={nvd ?? undefined} />
            <Field label={t("vuln.cvss")} value={entry.cvss != null ? `${entry.cvss} · ${entry.cvss_vector}` : "—"} mono wrap />
            <Field label={t("vuln.cwe")} value={entry.cwe || "—"} mono />
            <Field label={t("vuln.cveAnalysis.openFindings")} value={String(entry.open_findings)} />
          </div>
          <p className="text-sm text-[var(--text-secondary)] whitespace-pre-wrap">{entry.description}</p>
          {entry.references.length > 0 && (
            <ul className="space-y-0.5">
              {entry.references.map((r) => {
                const href = safeHref(r);
                return (
                  <li key={r} className="text-xs break-all">
                    {href ? <a href={href} target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] hover:underline">{r}</a> : r}
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      )}

      {isLoading ? (
        <Skeleton className="h-24 w-full" />
      ) : entry ? (
        <>
          <MarkdownContent content={text} />
          <p className="text-2xs text-[var(--text-muted)]">
            {t("vuln.cveAnalysis.meta", { ago: getTimeAgo(entry.analyzed_at, locale), model: entry.model || "—" })}
          </p>
        </>
      ) : (
        !analyze.isPending && (
          <p className="text-sm text-[var(--text-muted)]">
            {canAnalyze ? t("vuln.cveAnalysis.empty") : t("vuln.cveAnalysis.emptyReadOnly")}
          </p>
        )
      )}
    </section>
  );
}
