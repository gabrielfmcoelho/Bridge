"use client";

import DropdownMenu, { DropdownMenuGroup, DropdownMenuCheckboxItem } from "@/components/ui/DropdownMenu";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

export interface KpiOption {
  key: string;
  label: string;
  /** Options are listed under their group heading ("Indicadores", "Tags"). */
  group: string;
}

/**
 * "Customize" menu for a KPI strip: a checklist that stays open while ticking,
 * so several tiles can be swapped in one go. Selection order is kept.
 */
export default function KpiPicker({
  options,
  selected,
  onChange,
  onReset,
}: {
  options: KpiOption[];
  selected: string[];
  onChange: (keys: string[]) => void;
  onReset: () => void;
}) {
  const { t } = useLocale();
  const groups = [...new Set(options.map((o) => o.group))];
  const toggle = (key: string, on: boolean) =>
    onChange(on ? [...selected, key] : selected.filter((k) => k !== key));

  return (
    <DropdownMenu
      className="w-64"
      trigger={
        <Button size="sm" variant="ghost" aria-label={t("inventory.kpis.customize")}>
          <Icon path={ICON_PATHS.gear} className="w-3.5 h-3.5" />
          <span className="max-sm:hidden">{t("inventory.kpis.customize")}</span>
        </Button>
      }
    >
      <div className="max-h-80 overflow-y-auto">
        {groups.map((g) => (
          <DropdownMenuGroup key={g} title={g}>
            {options.filter((o) => o.group === g).map((o) => (
              <DropdownMenuCheckboxItem key={o.key} checked={selected.includes(o.key)} onCheckedChange={(on) => toggle(o.key, on)}>
                {o.label}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuGroup>
        ))}
      </div>
      <div className="border-t border-[var(--border-subtle)] p-1.5">
        <Button size="sm" variant="ghost" className="w-full" onClick={onReset}>
          {t("inventory.kpis.reset")}
        </Button>
      </div>
    </DropdownMenu>
  );
}
