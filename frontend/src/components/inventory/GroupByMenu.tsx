"use client";

import ToolbarSelect from "@/components/ui/ToolbarSelect";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";
import type { GroupEntity } from "@/lib/grouping";

/** Toolbar choice of which related entity a list is grouped by ("" = none). */
export default function GroupByMenu({
  options,
  value,
  onChange,
}: {
  options: GroupEntity[];
  value: GroupEntity | "";
  onChange: (v: GroupEntity | "") => void;
}) {
  const { t } = useLocale();
  const title = t("inventory.groupBy.title");
  return (
    <ToolbarSelect<GroupEntity | "">
      name={title}
      icon={ICON_PATHS.lanes}
      value={value}
      onChange={onChange}
      options={(["", ...options] as (GroupEntity | "")[]).map((o) => ({ value: o, label: t(o ? `inventory.groupBy.${o}` : "inventory.groupBy.none") }))}
      labelFor={(o) => (o.value ? `${title}: ${o.label}` : title)}
    />
  );
}
