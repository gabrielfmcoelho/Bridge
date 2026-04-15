"use client";

import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import type { Issue } from "@/lib/types";

interface IssuesTabProps {
  issues: Issue[];
  t: (key: string) => string;
}

export default function IssuesTab({ issues, t }: IssuesTabProps) {
  return (
    <div className="space-y-5 animate-fade-in">
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          {t("issue.title")}
        </h2>
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
                      ? "bg-red-400"
                      : issue.priority === "high"
                      ? "bg-amber-400"
                      : issue.priority === "medium"
                      ? "bg-cyan-400"
                      : "bg-[var(--text-faint)]"
                  }`}
                />
                <span className="text-[var(--text-primary)] flex-1 truncate">{issue.title}</span>
                <Badge>{issue.status}</Badge>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No issues reported</p>
        )}
      </Card>
    </div>
  );
}
