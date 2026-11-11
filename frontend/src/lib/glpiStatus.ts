// GLPI ticket status labels are translated client-side from status_slug —
// the backend's status_label is always Portuguese (glpi.StatusLabel in Go),
// so rendering it directly breaks the English UI. status_slug is the
// locale-agnostic key every ticket payload already carries.
export const GLPI_STATUS_LABEL_KEYS: Record<string, string> = {
  new: "glpi.statusNew",
  assigned: "glpi.statusAssigned",
  planned: "glpi.statusPlanned",
  waiting: "glpi.statusWaiting",
  solved: "glpi.statusSolved",
  closed: "glpi.statusClosed",
};

export function glpiStatusLabelKey(slug: string | undefined): string {
  return GLPI_STATUS_LABEL_KEYS[slug ?? ""] ?? "glpi.statusUnknown";
}
