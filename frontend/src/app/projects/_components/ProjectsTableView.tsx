"use client";

import { useRouter } from "next/navigation";
import Badge from "@/components/ui/Badge";
import type { Project } from "@/lib/types";

interface ProjectsTableViewProps {
  projects: Project[];
  t: (key: string) => string;
}

export default function ProjectsTableView({ projects, t }: ProjectsTableViewProps) {
  const router = useRouter();

  return (
    <div className="bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border border-[var(--border-subtle)] overflow-x-auto animate-fade-in">
      <table className="w-full text-sm min-w-[600px]">
        <thead>
          <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
            <th className="text-left px-4 py-3 font-semibold">{t("project.name")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("common.description")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("common.status")}</th>
            <th className="text-left px-4 py-3 font-semibold">{t("common.tags")}</th>
          </tr>
        </thead>
        <tbody>
          {projects.map((project, i) => (
            <tr
              key={project.id}
              className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              onClick={() => router.push(`/projects/${project.id}`)}
            >
              <td className="px-4 py-2.5 font-medium text-[var(--text-primary)]">{project.name}</td>
              <td className="px-4 py-2.5 text-[var(--text-secondary)] max-w-[200px] truncate">{project.description || "-"}</td>
              <td className="px-4 py-2.5"><Badge variant="situacao" situacao={project.situacao} dot>{project.situacao}</Badge></td>
              <td className="px-4 py-2.5">
                <div className="flex flex-wrap gap-1">
                  {project.tags?.slice(0, 3).map((tag) => <Badge key={tag}>{tag}</Badge>)}
                  {project.tags && project.tags.length > 3 && <span className="text-[10px] text-[var(--text-faint)]">+{project.tags.length - 3}</span>}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
