"use client";

import { use } from "react";
import RequestDetail from "./RequestDetail";

export default function RequestDetailPage(props: { params: Promise<{ id: string }> }) {
  const { id } = use(props.params);
  return <RequestDetail id={parseInt(id, 10)} />;
}
