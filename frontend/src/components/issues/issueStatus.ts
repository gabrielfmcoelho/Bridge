export const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;

export function getStatusLabels(t: (k: string) => string): Record<string, string> {
  return { backlog: t("issue.backlog"), todo: t("issue.todo"), in_progress: t("issue.inProgress"), review: t("issue.review"), done: t("issue.done") };
}
