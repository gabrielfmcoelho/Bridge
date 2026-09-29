"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { releasesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Drawer from "@/components/ui/Drawer";
import ReleaseTimeline from "@/components/detail/ReleaseTimeline";
import { ICON_PATHS } from "@/lib/icon-paths";
import ReleaseForm from "./ReleaseForm";
import type { Release } from "@/lib/types";

type R = Release & { issue_ids?: number[] };

/** The Lançamentos tab: the project's release timeline and its drawer form. */
export default function ProjectReleases({ projectId, canEdit, canDelete }: { projectId: number; canEdit: boolean; canDelete: boolean }) {
  const { t, locale } = useLocale();
  const queryClient = useQueryClient();
  const key = ["releases", projectId];
  const { data: releases = [], isLoading } = useQuery({ queryKey: key, queryFn: () => releasesAPI.list(projectId) });
  // null = closed, "new" = create, else the release being edited.
  const [editing, setEditing] = useState<R | "new" | null>(null);
  const close = () => setEditing(null);
  const done = () => { close(); queryClient.invalidateQueries({ queryKey: key }); };

  return (
    <>
      <SectionCard
        as="h3"
        title={t("release.title")}
        count={releases.length}
        body="flush"
        empty={!isLoading && releases.length === 0 ? t("release.emptyProject") : undefined}
        controls={canEdit && (
          <IconButton onClick={() => setEditing("new")} label={t("release.create")}><Icon path={ICON_PATHS.plus} /></IconButton>
        )}
      >
        <ReleaseTimeline releases={releases} onSelect={canEdit ? setEditing : undefined} t={t} locale={locale} />
      </SectionCard>

      <Drawer open={editing !== null} onClose={close}
        title={editing && editing !== "new" ? t("form.editTitle", { name: editing.title }) : t("release.create")}>
        {editing !== null && (
          <ReleaseForm projectId={projectId} release={editing === "new" ? undefined : editing}
            onDone={done} onCancel={close} canDelete={canDelete} />
        )}
      </Drawer>
    </>
  );
}
