"use client";

import { use } from "react";
import DnsDetail from "./DnsDetail";

export default function DnsDetailPage(props: { params: Promise<{ id: string }> }) {
  const { id } = use(props.params);
  return <DnsDetail id={parseInt(id)} />;
}
