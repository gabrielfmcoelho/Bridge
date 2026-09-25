"use client";

import Badge from "@/components/ui/Badge";
import type { Issue } from "@/lib/types";
import SectionCard from "@/components/ui/SectionCard";

interface IssuesTabProps {
  issues: Issue[];
  t: (key: string) => string;
}

export default function IssuesTab({ issues, t }: IssuesTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      <SectionCard title={t("issue.title")} count={issues.length}>
        {issues.length > 0 ? (
          <div className="space-y-1">
            {issues.map((issue) => (
              <div
                key={issue.id}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)]"
              >
                <span
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    issue.priority === "critical"
                      ? "bg-[var(--danger)]"
                      : issue.priority === "high"
                      ? "bg-[var(--warning)]"
                      : issue.priority === "medium"
                      ? "bg-[var(--cyan)]"
                      : "bg-[var(--text-faint)]"
                  }`}
                />
                <span className="text-[var(--text-primary)] flex-1 truncate">{issue.title}</span>
                <Badge>{issue.status}</Badge>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">{t("issue.noneReported")}</p>
        )}
      </SectionCard>
    </div>
  );
}
