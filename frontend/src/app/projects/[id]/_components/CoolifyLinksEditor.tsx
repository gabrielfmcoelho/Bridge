"use client";

import { useQuery } from "@tanstack/react-query";
import Select from "@/components/ui/Select";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { coolifyAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ProjectCoolifyLink } from "@/lib/types";

interface Props {
  value: ProjectCoolifyLink[];
  onChange: (rows: ProjectCoolifyLink[]) => void;
}

/**
 * The Coolify projects/environments that belong to this Bridge project. The
 * Coolify sync gives their services this project when they have none.
 */
export default function CoolifyLinksEditor({ value, onChange }: Props) {
  const { t } = useLocale();
  const { data: projects = [], isError } = useQuery({ queryKey: ["coolify-projects"], queryFn: coolifyAPI.projects, staleTime: 60_000, retry: false });
  const set = (i: number, patch: Partial<ProjectCoolifyLink>) => onChange(value.map((r, j) => (j === i ? { ...r, ...patch } : r)));
  const projectOptions = projects.map((p) => ({ value: p.name, label: p.name }));

  if (isError) return <p className="text-xs text-[var(--text-muted)]">{t("project.coolifyUnavailable")}</p>;
  return (
    <div className="space-y-2">
      {value.map((r, i) => {
        const envs = projects.find((p) => p.name === r.coolify_project)?.environments ?? [];
        return (
          <div key={i} className="flex flex-wrap sm:flex-nowrap items-start gap-2">
            <div className="w-full sm:flex-1 min-w-0">
              <Select aria-label={t("project.coolifyProject")} value={r.coolify_project} options={projectOptions}
                onChange={(e) => set(i, { coolify_project: e.target.value, coolify_environment: "" })} />
            </div>
            <div className="w-full sm:w-48 shrink-0">
              <Select aria-label={t("project.coolifyEnvironment")} value={r.coolify_environment}
                options={[{ value: "", label: t("project.coolifyAllEnvironments") }, ...envs.map((e) => ({ value: e, label: e }))]}
                onChange={(e) => set(i, { coolify_environment: e.target.value })} />
            </div>
            <IconButton label={t("common.removeItem", { label: r.coolify_project || "Coolify" })} onClick={() => onChange(value.filter((_, j) => j !== i))}>
              <Icon path={ICON_PATHS.close} />
            </IconButton>
          </div>
        );
      })}
      <Button type="button" size="sm" variant="secondary" onClick={() => onChange([...value, { coolify_project: "", coolify_environment: "" }])}>
        {t("project.coolifyAddMapping")}
      </Button>
    </div>
  );
}
