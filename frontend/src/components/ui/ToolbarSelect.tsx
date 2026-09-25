"use client";

import DropdownMenu, { DropdownMenuGroup, DropdownMenuRadioItem } from "./DropdownMenu";
import ToolbarActionButton from "./ToolbarActionButton";
import Icon from "./Icon";

export interface ToolbarSelectOption<V extends string> {
  value: V;
  label: string;
  icon?: string;
}

/**
 * A choice among a few options in a list toolbar (view: cards/table, group
 * by: none/service/...). Same button as the toolbar's actions; its label is
 * the current choice, so the toolbar says what the list is showing.
 */
export default function ToolbarSelect<V extends string>({
  name,
  icon,
  value,
  options,
  onChange,
  labelFor = (o) => o.label,
}: {
  /** What is being chosen ("Exibição"), for the accessible name. */
  name: string;
  /** Button icon when the current option has none. */
  icon: string;
  value: V;
  options: ToolbarSelectOption<V>[];
  onChange: (value: V) => void;
  /** Button text for the current option (default: its label). */
  labelFor?: (option: ToolbarSelectOption<V>) => string;
}) {
  const current = options.find((o) => o.value === value) ?? options[0];
  return (
    <DropdownMenu
      trigger={
        <ToolbarActionButton
          icon={current.icon ?? icon}
          label={labelFor(current)}
          title={`${name}: ${current.label}`}
          aria-label={`${name}: ${current.label}`}
          hideLabel="md"
        />
      }
    >
      <DropdownMenuGroup title={name}>
        {options.map((o) => (
          <DropdownMenuRadioItem
            key={o.value}
            checked={o.value === value}
            onSelect={() => onChange(o.value)}
            elemBefore={o.icon && <Icon path={o.icon} className="w-4 h-4" />}
          >
            {o.label}
          </DropdownMenuRadioItem>
        ))}
      </DropdownMenuGroup>
    </DropdownMenu>
  );
}
