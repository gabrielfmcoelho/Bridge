"use client";

import LinkedApisCard from "@/app/atlas/apis/_components/LinkedApisCard";

/** A project's APIs: linked directly or through one of its services. */
export default function ProjectApisTab({ projectId, canEdit }: { projectId: number; canEdit: boolean }) {
  return <LinkedApisCard filter={{ project_id: projectId }} canEdit={canEdit} prefill={{ projectIds: [projectId] }} />;
}
