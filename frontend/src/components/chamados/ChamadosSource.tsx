"use client";

import { useRouter } from "next/navigation";
import { useLocale } from "@/contexts/LocaleContext";
import { NAV_ICONS } from "@/lib/icon-paths";
import Icon from "@/components/ui/Icon";
import PillButton from "@/components/ui/PillButton";

const SOURCES = [
  { key: "glpi", href: "/chamados", label: "chamados.source.glpi", icon: NAV_ICONS.Ticket },
  { key: "glpi-sead", href: "/chamados-sead", label: "chamados.source.glpiSead", icon: NAV_ICONS.Ticket },
  { key: "bridge", href: "/requests", label: "chamados.source.bridge", icon: NAV_ICONS.ClipboardList },
] as const;

/** Chamados is one sidebar entry with three ways to ask for something: GLPI
 *  (ETIPI) tickets (/chamados), GLPI SEAD/NTGD tickets (/chamados-sead), and
 *  Bridge's own requests (/requests). This switch sits under the title of
 *  all three pages. */
export default function ChamadosSource({ active }: { active: "glpi" | "glpi-sead" | "bridge" }) {
  const { t } = useLocale();
  const router = useRouter();
  return (
    <div role="group" aria-label={t("chamados.source.label")} className="flex flex-wrap gap-1.5">
      {SOURCES.map((s) => (
        <PillButton
          key={s.key}
          active={s.key === active}
          aria-current={s.key === active ? "page" : undefined}
          onClick={() => s.key !== active && router.push(s.href)}
          lead={<Icon path={s.icon} className="w-3.5 h-3.5" strokeWidth={1.5} />}
        >
          {t(s.label)}
        </PillButton>
      ))}
    </div>
  );
}
