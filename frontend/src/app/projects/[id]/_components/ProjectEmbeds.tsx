"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { projectEmbedsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton, { IconLink } from "@/components/ui/IconButton";
import PillButton from "@/components/ui/PillButton";
import Icon from "@/components/ui/Icon";
import Drawer from "@/components/ui/Drawer";
import { ICON_PATHS } from "@/lib/icon-paths";
import EmbedForm from "./EmbedForm";
import type { ProjectEmbed } from "@/lib/types";

export const projectEmbedsKey = (projectId: number) => ["project-embeds", projectId];

/**
 * The Observabilidade tab: the project's BIs and tools, one at a time in an
 * iframe. Sites that refuse to be framed still open in a new tab.
 */
export default function ProjectEmbeds({ projectId, canEdit }: { projectId: number; canEdit: boolean }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const key = projectEmbedsKey(projectId);
  const { data: embeds = [], isLoading } = useQuery({ queryKey: key, queryFn: () => projectEmbedsAPI.list(projectId) });
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [editing, setEditing] = useState<ProjectEmbed | "new" | null>(null);
  const current = embeds.find((e) => e.id === selectedId) ?? embeds[0];

  const close = () => setEditing(null);
  const done = () => { close(); queryClient.invalidateQueries({ queryKey: key }); };

  return (
    <>
      <SectionCard
        as="h3"
        title={t("embed.title")}
        count={embeds.length}
        body="flush"
        empty={!isLoading && embeds.length === 0 ? t("embed.empty") : undefined}
        controls={<>
          {current && <IconLink variant="default" href={current.url} label={t("embed.openNewTab")}><Icon path={ICON_PATHS.externalLink} /></IconLink>}
          {canEdit && current && <IconButton onClick={() => setEditing(current)} label={t("common.edit")}><Icon path={ICON_PATHS.editPencil} /></IconButton>}
          {canEdit && <IconButton onClick={() => setEditing("new")} label={t("embed.add")}><Icon path={ICON_PATHS.plus} /></IconButton>}
        </>}
      >
        {current && (
          <>
            {embeds.length > 1 && (
              <div className="flex flex-wrap gap-1.5 px-5 pb-3">
                {embeds.map((e) => (
                  <PillButton key={e.id} size="sm" active={e.id === current.id} onClick={() => setSelectedId(e.id)}>{e.title}</PillButton>
                ))}
              </div>
            )}
            <iframe
              key={current.id}
              src={current.url}
              title={current.title}
              loading="lazy"
              // Same sandbox as the Ferramentas page: the BI runs its scripts,
              // but can't navigate the Bridge tab.
              sandbox="allow-same-origin allow-scripts allow-popups allow-forms"
              className="block w-full border-t border-[var(--border-subtle)] bg-[var(--bg-base)] rounded-b-[var(--radius-lg)]"
              style={{ height: current.height }}
            />
          </>
        )}
      </SectionCard>

      <Drawer open={editing !== null} onClose={close}
        title={editing && editing !== "new" ? t("form.editTitle", { name: editing.title }) : t("embed.add")}>
        {editing !== null && (
          <EmbedForm projectId={projectId} embed={editing === "new" ? undefined : editing} onDone={done} onCancel={close} />
        )}
      </Drawer>
    </>
  );
}
