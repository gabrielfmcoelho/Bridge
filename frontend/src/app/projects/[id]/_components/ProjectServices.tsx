"use client";

import SectionCard from "@/components/ui/SectionCard";
import { StatusText } from "@/components/ui/SituacaoText";
import Icon from "@/components/ui/Icon";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { useLocale } from "@/contexts/LocaleContext";
import { useHostNames } from "@/hooks/useHostNames";
import { ICON_PATHS } from "@/lib/icon-paths";
import { serviceTitle } from "@/lib/serviceDisplay";
import type { Service } from "@/lib/types";

/** What makes up the project: its services as rows — title, where it runs,
 *  category, runtime state. Hosts and DNS live in Topologia. */
export default function ProjectServices({ services }: { services: Service[] }) {
  const { t } = useLocale();
  const hostNames = useHostNames();
  return (
    <SectionCard as="h3" title={t("project.servicesTitle")} count={services.length} body="flush" empty={services.length === 0 ? t("project.noServices") : undefined}>
      <RowList>
        {services.map((s) => {
          const { title, mono } = serviceTitle(s);
          const hosts = (s.host_ids ?? []).map((id) => hostNames.get(id)).filter(Boolean).join(", ");
          return (
            <ListRow key={s.id} href={`/services/${s.id}`}>
              <Icon path={ICON_PATHS.serverStack} className="w-3.5 h-3.5 shrink-0 text-[var(--accent)]" />
              <RowText
                title={title}
                mono={mono}
                meta={<>
                  {hosts && <span className="truncate">{hosts}</span>}
                  {s.service_kind && <span>{t(`service.kind.${s.service_kind}`)}</span>}
                </>}
              />
              {s.container_status && (
                <StatusText color={s.container_status === "online" ? "var(--success)" : "var(--text-muted)"} on={s.container_status === "online"} label={t(`service.status.${s.container_status}`)} />
              )}
            </ListRow>
          );
        })}
      </RowList>
    </SectionCard>
  );
}
