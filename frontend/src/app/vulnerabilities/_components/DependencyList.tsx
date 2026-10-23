"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Pagination from "@/components/ui/Pagination";
import EmptyState from "@/components/ui/EmptyState";
import StatusAlert from "@/components/ui/StatusAlert";
import Badge from "@/components/ui/Badge";
import Lozenge from "@/components/ui/Lozenge";
import Field from "@/components/ui/Field";
import SectionHeading from "@/components/ui/SectionHeading";
import { SkeletonTable } from "@/components/ui/Skeleton";
import { tableClasses } from "@/components/ui/Table";
import { useLocale } from "@/contexts/LocaleContext";
import { vulnAPI } from "@/lib/api";
import { VULN_SEVERITY_BADGE } from "@/lib/alert-colors";
import type { Dependency } from "@/lib/types";

const PER_PAGE = 50;

export type DependencyKindFilter = "components" | "packages" | "all";
export type DependencyFlag = "" | "cves" | "eol";

const KINDS: Record<DependencyKindFilter, string> = { components: "os,runtime", packages: "package", all: "os,runtime,package" };

/** The Dependencies tab: every OS / runtime / package version the host scans
 *  found, on the host or inside a container, with its CVE count and EOL flag.
 *  A row opens its advisories and the findings they produced. */
export default function DependencyList({ search, kind, flag, onOpenFinding }: {
  search: string;
  kind: DependencyKindFilter;
  flag: DependencyFlag;
  onOpenFinding: (id: number) => void;
}) {
  const { t } = useLocale();
  const key = [search, kind, flag].join("|");
  const [paged, setPaged] = useState({ key, page: 1 });
  const page = paged.key === key ? paged.page : 1;
  const [openId, setOpenId] = useState<number | null>(null);

  const params: Record<string, string> = { page: String(page), per_page: String(PER_PAGE), kind: KINDS[kind] };
  if (search) params.search = search;
  if (flag === "cves") params.has_cves = "true";
  if (flag === "eol") params.eol = "true";
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ["vulnerabilities", "dependencies", params],
    queryFn: () => vulnAPI.dependencies(params),
  });
  const rows = data?.data ?? [];
  const total = data?.meta.total ?? 0;

  return (
    <>
      {isLoading ? (
        <SkeletonTable rows={8} />
      ) : isError ? (
        <StatusAlert variant="error">{(error as Error)?.message}</StatusAlert>
      ) : rows.length === 0 ? (
        <EmptyState icon="search" title={t("vuln.empty.dependencies")} description={t("vuln.deps.emptyHint")} />
      ) : (
        <div className="space-y-3">
          <div className={tableClasses.wrapper}>
            <table className={tableClasses.table}>
              <thead>
                <tr className={tableClasses.headRow}>
                  <th className={tableClasses.th}>{t("vuln.deps.component")}</th>
                  <th className={tableClasses.th}>{t("vuln.deps.version")}</th>
                  <th className={tableClasses.th}>{t("vuln.deps.where")}</th>
                  <th className={tableClasses.th}>{t("vuln.deps.versionFrom")}</th>
                  <th className={tableClasses.th}>{t("vuln.deps.eol")}</th>
                  <th className={tableClasses.th}>{t("vuln.deps.cves")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((d, i) => (
                  <tr key={d.id} className={`${tableClasses.row} cursor-pointer ${i % 2 === 1 ? tableClasses.rowAlt : ""}`} onClick={() => setOpenId(d.id)}>
                    <td className={tableClasses.td}>
                      <span className="inline-flex items-center gap-2">
                        <Lozenge appearance={d.kind === "os" ? "inprogress" : d.kind === "runtime" ? "new" : "default"}>{t(`vuln.deps.kind.${d.kind}`)}</Lozenge>
                        <span className="font-medium text-[var(--text-primary)]">{d.name}</span>
                      </span>
                    </td>
                    <td className={`${tableClasses.td} font-mono text-xs`}>{d.version}</td>
                    <td className={tableClasses.td}><Where d={d} /></td>
                    <td className={`${tableClasses.td} text-[var(--text-muted)] whitespace-nowrap`}>{t(`vuln.deps.source.${d.version_source}`)}</td>
                    <td className={tableClasses.td}>{d.eol ? <Badge color="danger">{d.eol_date || t("vuln.deps.eolYes")}</Badge> : <span className="text-[var(--text-muted)]">—</span>}</td>
                    <td className={tableClasses.td}><CveCell d={d} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={page} totalPages={Math.max(1, Math.ceil(total / PER_PAGE))} total={total} perPage={PER_PAGE}
            onChange={(p) => setPaged({ key, page: p })} />
        </div>
      )}
      <DependencyDrawer id={openId} onClose={() => setOpenId(null)} onOpenFinding={onOpenFinding} />
    </>
  );
}

/** "vm-app · web (nginx:1.25)" — the host, and the container when there is one. */
function Where({ d }: { d: Dependency }) {
  const { t } = useLocale();
  return (
    <span className="text-[var(--text-secondary)]">
      <Link href={`/hosts/${d.host_slug}`} onClick={(e) => e.stopPropagation()} className="text-[var(--accent)] hover:underline">{d.host_nickname}</Link>
      {d.container ? (
        <> · {d.service_id ? (
          <Link href={`/services/${d.service_id}`} onClick={(e) => e.stopPropagation()} className="text-[var(--accent)] hover:underline">{d.container}</Link>
        ) : d.container}
          {d.image && <span className="ml-1 font-mono text-xs text-[var(--text-muted)]">{d.image}</span>}
        </>
      ) : <span className="ml-1 text-xs text-[var(--text-muted)]">{t("vuln.deps.onHost")}</span>}
    </span>
  );
}

function CveCell({ d }: { d: Dependency }) {
  const { t } = useLocale();
  if (!d.lookup) {
    // The OS has an EOL, not CVEs; a probed runtime without a lookup has its
    // CVEs on the distro package that ships it (e.g. openssl → libssl3).
    if (d.kind === "os") return <span className="text-[var(--text-muted)]">—</span>;
    return <span className="text-[var(--text-muted)]">{d.kind === "runtime" && d.version_source === "probe" ? t("vuln.deps.viaPackage") : t("vuln.deps.noLookup")}</span>;
  }
  if (!d.evaluated_at) return <span className="text-[var(--text-muted)]">{t("vuln.deps.notEvaluated")}</span>;
  if (d.cve_count === 0) return <span className="text-[var(--text-muted)]">0</span>;
  return (
    <span className="inline-flex items-center gap-2">
      <span className="font-mono">{d.cve_count}</span>
      {d.max_severity && <Badge color={VULN_SEVERITY_BADGE[d.max_severity]}>{t(`vuln.severityLevel.${d.max_severity}`)}</Badge>}
    </span>
  );
}

function DependencyDrawer({ id, onClose, onOpenFinding }: { id: number | null; onClose: () => void; onOpenFinding: (id: number) => void }) {
  const { t, formatDate } = useLocale();
  const { data: d } = useQuery({ queryKey: ["vulnerabilities", "dependency", id], queryFn: () => vulnAPI.dependency(id!), enabled: id != null });
  return (
    <Drawer open={id != null} onClose={onClose} wide title={d ? `${d.name} ${d.version}` : ""}>
      {d && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 gap-4">
            <Field label={t("vuln.deps.kindLabel")} value={t(`vuln.deps.kind.${d.kind}`)} />
            <Field label={t("vuln.deps.versionFrom")} value={t(`vuln.deps.source.${d.version_source}`)} />
            <Field label={t("vuln.deps.host")} value={d.host_nickname} />
            <Field label={t("vuln.deps.container")} value={d.container || t("vuln.deps.onHost")} />
            {d.image && <Field className="col-span-2" label={t("vuln.deps.image")} value={d.image} mono />}
            <Field label={t("vuln.deps.lookup")} value={d.lookup ? `${d.lookup.toUpperCase()} · ${d.ecosystem || d.lookup_name}` : t("vuln.deps.noLookup")} mono={!!d.lookup} />
            <Field label={t("vuln.deps.lookupVersion")} value={d.lookup_version} mono />
            <Field label={t("vuln.deps.eol")} value={d.eol ? (d.eol_date || t("vuln.deps.eolYes")) : d.eol_product ? t("vuln.deps.eolNo") : ""} />
            <Field label={t("vuln.deps.evaluatedAt")} value={d.evaluated_at ? formatDate(d.evaluated_at) : t("vuln.deps.notEvaluated")} />
            <Field label={t("vuln.firstSeen")} value={formatDate(d.first_seen_at)} />
            <Field label={t("vuln.lastSeen")} value={formatDate(d.last_seen_at)} />
          </div>

          <div>
            <SectionHeading as="h3">{t("vuln.deps.vulns")}</SectionHeading>
            {d.vulns.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)]">{d.evaluated_at ? t("vuln.deps.noVulns") : t("vuln.deps.notEvaluatedHint")}</p>
            ) : (
              <div className={tableClasses.wrapper}>
                <table className={tableClasses.table}>
                  <thead>
                    <tr className={tableClasses.headRow}>
                      <th className={tableClasses.th}>{t("vuln.cve")}</th>
                      <th className={tableClasses.th}>{t("vuln.severity")}</th>
                      <th className={tableClasses.th}>{t("vuln.cvss")}</th>
                      <th className={tableClasses.th}>{t("vuln.deps.fixed")}</th>
                      <th className={tableClasses.th}>{t("vuln.cveAnalysis.description")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {d.vulns.map((v, i) => {
                      const finding = d.findings.find((f) => f.cve === v.id);
                      return (
                        <tr key={v.id} className={`${tableClasses.row} ${finding ? "cursor-pointer" : ""} ${i % 2 === 1 ? tableClasses.rowAlt : ""}`}
                          onClick={finding ? () => onOpenFinding(finding.id) : undefined}>
                          <td className={`${tableClasses.td} font-mono text-xs whitespace-nowrap`}>{v.id}</td>
                          <td className={tableClasses.td}><Badge color={VULN_SEVERITY_BADGE[v.severity]}>{t(`vuln.severityLevel.${v.severity}`)}</Badge></td>
                          <td className={tableClasses.td}>{v.cvss || "—"}</td>
                          <td className={`${tableClasses.td} font-mono text-xs`}>{v.fixed || "—"}</td>
                          <td className={`${tableClasses.td} max-w-0 w-[50%]`}><span className="block truncate text-[var(--text-secondary)]" title={v.summary}>{v.summary}</span></td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      )}
    </Drawer>
  );
}
