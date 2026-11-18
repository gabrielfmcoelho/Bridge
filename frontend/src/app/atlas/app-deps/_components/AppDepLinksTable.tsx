import Link from "next/link";
import Icon from "@/components/ui/Icon";
import { tableClasses } from "@/components/ui/Table";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { AppDepLinkRow } from "@/lib/appDeps";
import { appDepsHref } from "@/lib/appDeps";
import CriticalityBadge from "./CriticalityBadge";

const dash = <span className="text-[var(--text-faint)]">–</span>;

/** The links one CI declares: what it consumes (upstream) and who consumes it (downstream). */
export default function AppDepLinksTable({ rows }: { rows: AppDepLinkRow[] }) {
  const { t } = useLocale();
  if (rows.length === 0) return <p className="py-3 text-sm text-[var(--text-muted)]">{t("appDeps.links.empty")}</p>;
  const th = tableClasses.compact.th;
  const td = `${tableClasses.compact.td} align-top`;
  return (
    <div className="overflow-x-auto">
      <table className={tableClasses.compact.table}>
        <thead>
          <tr className={tableClasses.compact.headRow}>
            <th className={th}>{t("appDeps.links.direction")}</th>
            <th className={th}>{t("appDeps.links.other")}</th>
            <th className={th}>{t("appDeps.links.component")}</th>
            <th className={th}>{t("appDeps.links.local")}</th>
            <th className={th}>{t("appDeps.links.technology")}</th>
            <th className={th}>{t("appDeps.links.endpoint")}</th>
            <th className={th}>{t("appDeps.links.criticality")}</th>
            <th className={th}>{t("appDeps.links.description")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={`${r.direction}-${r.other}-${r.component ?? ""}-${r.localComponent ?? ""}-${i}`} className={tableClasses.compact.row}>
              <td className={`${td} whitespace-nowrap`}>
                <span className="inline-flex items-center gap-1 text-xs text-[var(--text-secondary)]">
                  <Icon path={r.direction === "upstream" ? ICON_PATHS.arrowRight : ICON_PATHS.arrowLeft} className="w-3.5 h-3.5" />
                  {t(r.direction === "upstream" ? "appDeps.links.upstream" : "appDeps.links.downstream")}
                </span>
              </td>
              <td className={`${td} font-mono text-xs whitespace-nowrap`}>
                {r.catalogued ? (
                  <Link href={appDepsHref(r.other)} className="text-[var(--accent)] hover:underline">{r.other}</Link>
                ) : (
                  <span title={t("appDeps.links.notCatalogued")}>{r.other}</span>
                )}
                {r.via && <span className="block text-2xs text-[var(--text-muted)]">{t("appDeps.links.via", { via: r.via })}</span>}
              </td>
              <td className={`${td} font-mono text-xs`}>{r.component || dash}</td>
              <td className={`${td} font-mono text-xs`}>{r.localComponent || dash}</td>
              <td className={`${td} font-mono text-xs`}>{r.technology || dash}</td>
              <td className={`${td} font-mono text-xs`}>{r.endpoint || dash}</td>
              <td className={td}><CriticalityBadge value={r.criticality} /></td>
              <td className={`${td} text-xs text-[var(--text-secondary)] min-w-48`}>{r.description || dash}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
