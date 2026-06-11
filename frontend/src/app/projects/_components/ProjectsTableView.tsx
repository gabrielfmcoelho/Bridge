"use client";

import { useRouter } from "next/navigation";
import SortableTable from "@/components/ui/SortableTable";
import Pagination from "@/components/ui/Pagination";
import Badge from "@/components/ui/Badge";
import type { Project } from "@/lib/types";

// Server-driven table (inventory pagination). `projects` is ONE server page
// (filtered+sorted+limited by the parent's paginated query), `total` is the full
// match count for the pager, and column-header clicks drive the page-level sort
// via onSortChange (→ server refetch). description/tags aren't server-sortable.
const PER_PAGE = 20;
type Sort = { field: string; direction: "asc" | "desc" };
type ColKey = "name" | "description" | "situacao" | "tags";

interface ProjectsTableViewProps {
  projects: Project[];
  total: number;
  tablePage: number;
  onPageChange: (page: number) => void;
  sort: Sort;
  onSortChange: (s: Sort) => void;
  t: (key: string) => string;
}

export default function ProjectsTableView({ projects, total, tablePage, onPageChange, sort, onSortChange, t }: ProjectsTableViewProps) {
  const router = useRouter();
  return (
    <div className="animate-fade-in">
      <SortableTable
        columns={[
          { key: "name" as ColKey, label: t("project.name") },
          { key: "description" as ColKey, label: t("common.description"), sortable: false },
          { key: "situacao" as ColKey, label: t("common.status") },
          { key: "tags" as ColKey, label: t("common.tags"), sortable: false },
        ]}
        sortKey={sort.field as ColKey}
        sortDir={sort.direction}
        onSortChange={(key, dir) => {
          if (key === "description" || key === "tags") return; // not server-sortable
          onSortChange({ field: key, direction: dir });
        }}
      >
        {() =>
          projects.map((project, i) => (
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
          ))
        }
      </SortableTable>
      <Pagination page={tablePage} totalPages={Math.max(1, Math.ceil(total / PER_PAGE))} total={total} perPage={PER_PAGE} onChange={onPageChange} />
    </div>
  );
}
