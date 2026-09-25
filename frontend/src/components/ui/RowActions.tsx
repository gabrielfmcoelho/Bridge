"use client";

import DropdownMenu, { DropdownMenuItem } from "./DropdownMenu";
import IconButton from "./IconButton";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

export interface RowAction {
  label: string;
  icon?: string;
  onClick: () => void;
  danger?: boolean;
  hidden?: boolean;
}

/**
 * A table row's actions behind one "…" button (ADS dynamic table). The
 * trigger's name carries the row ("Ações: web-01"), so a screen reader can
 * tell twenty identical buttons apart. Destructive items go last.
 */
export default function RowActions({ name, actions }: { name: string; actions: RowAction[] }) {
  const { t } = useLocale();
  const shown = actions.filter((a) => !a.hidden);
  if (shown.length === 0) return null;
  return (
    <DropdownMenu
      trigger={
        <IconButton label={`${t("common.actions")}: ${name}`}>
          <Icon path={ICON_PATHS.moreHorizontal} className="w-4 h-4" strokeWidth={3} />
        </IconButton>
      }
    >
      <div className="py-1">
        {[...shown.filter((a) => !a.danger), ...shown.filter((a) => a.danger)].map((a) => (
          <DropdownMenuItem
            key={a.label}
            danger={a.danger}
            onClick={a.onClick}
            elemBefore={a.icon && <Icon path={a.icon} className="w-4 h-4" />}
          >
            {a.label}
          </DropdownMenuItem>
        ))}
      </div>
    </DropdownMenu>
  );
}
