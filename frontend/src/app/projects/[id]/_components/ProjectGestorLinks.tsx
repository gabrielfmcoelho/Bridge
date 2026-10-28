"use client";

import SectionCard from "@/components/ui/SectionCard";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { useLocale } from "@/contexts/LocaleContext";
import type { ProjectGestorLink } from "@/lib/types";

/** The Gestor projects this project refers to (id + name as of the last save). Hidden when none. */
export default function ProjectGestorLinks({ links }: { links: ProjectGestorLink[] }) {
  const { t } = useLocale();
  if (links.length === 0) return null;
  return (
    <SectionCard as="h3" title={t("project.gestorTitle")} count={links.length} body="flush">
      <RowList>
        {links.map((l) => (
          <ListRow key={l.gestor_projeto_id}>
            <RowText title={l.gestor_projeto_nome || `#${l.gestor_projeto_id}`} meta={<span>#{l.gestor_projeto_id}</span>} />
          </ListRow>
        ))}
      </RowList>
    </SectionCard>
  );
}
