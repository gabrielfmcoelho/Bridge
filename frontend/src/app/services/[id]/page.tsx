"use client";

import { use } from "react";
import ServiceDetail from "./ServiceDetail";

export default function ServiceDetailPage(props: { params: Promise<{ id: string }> }) {
  const { id } = use(props.params);
  return <ServiceDetail id={parseInt(id, 10)} />;
}
