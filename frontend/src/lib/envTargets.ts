export interface EnvTarget {
  scope: string;
  parent_id?: number;
}

// buildEnvTargets assembles the multi-target array for an env_var bundle synced
// to a project plus zero or more of that project's services. The service rows
// are always the same project's services (the UI checklist guarantees it; the
// backend guardServiceInProject is the safety net), so this stays a plain map.
//
// serviceIDs empty -> a single projeto target (caller may instead skip targets
// entirely and use the legacy single scope/parent path; both are equivalent).
export function buildEnvTargets(projectID: number, serviceIDs: number[]): EnvTarget[] {
  return [
    { scope: "projeto", parent_id: projectID },
    ...serviceIDs.map((id) => ({ scope: "service", parent_id: id })),
  ];
}
