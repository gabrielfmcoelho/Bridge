"use client";

import { use } from "react";
import HostDetail from "./HostDetail";

export default function HostDetailPage(props: { params: Promise<{ slug: string }> }) {
  const { slug } = use(props.params);
  return <HostDetail slug={slug} />;
}
