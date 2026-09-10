"use client";

import Button from "./Button";
import DropdownMenu, { DropdownMenuItem } from "./DropdownMenu";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

export default function SortDropdown<K extends string>({
  options,
  value,
  direction,
  onChange,
}: {
  options: { key: K; label: string }[];
  value: K;
  direction: "asc" | "desc";
  onChange: (key: K, direction: "asc" | "desc") => void;
}) {
  return (
    <div className="flex items-center gap-1">
      <DropdownMenu
        trigger={
          <Button size="sm" variant="secondary">
            {options.find((o) => o.key === value)?.label}
            <Icon path={ICON_PATHS.chevronDown} className="w-3 h-3 ml-0.5" />
          </Button>
        }
      >
        {options.map((opt) => (
          <DropdownMenuItem
            key={opt.key}
            active={opt.key === value}
            className="text-xs"
            onClick={() => onChange(opt.key, opt.key === value ? direction : "asc")}
          >
            {opt.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenu>
      <Button size="sm" variant="secondary" onClick={() => onChange(value, direction === "asc" ? "desc" : "asc")}>
        <Icon path={ICON_PATHS.chevronUp} className={`w-3.5 h-3.5 transition-transform ${direction === "desc" ? "rotate-180" : ""}`} />
      </Button>
    </div>
  );
}
