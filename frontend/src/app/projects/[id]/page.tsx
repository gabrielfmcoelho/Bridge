"use client";

import { use } from "react";
import ProjectDetail from "./ProjectDetail";

export default function ProjectDetailPage(props: { params: Promise<{ id: string }> }) {
  const { id } = use(props.params);
  return <ProjectDetail id={parseInt(id, 10)} />;
}
