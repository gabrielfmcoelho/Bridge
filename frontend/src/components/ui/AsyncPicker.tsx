"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import * as Popover from "@radix-ui/react-popover";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useLocale } from "@/contexts/LocaleContext";

export interface AsyncPickerItem {
  id: number;
  label: string;
  secondary?: string;
  color?: string;
  /** Ancestor names (root → immediate parent), for tree-shaped itemtypes
   *  (GLPI categories, locations, ...). When present, the list renders
   *  non-selectable ancestor header rows above the item — same convention
   *  GLPI's own dropdown picker uses — deduplicated against the previous
   *  row's path so a shared ancestor only prints once. */
  path?: string[];
}

// Single-select variant.
interface SingleProps {
  multi?: false;
  value: number | null;
  onChange: (item: AsyncPickerItem | null) => void;
  selectedLabel?: string;
  // Multi-only props are not valid here.
  selectedItems?: never;
}

// Multi-select variant — the caller owns the selection list so the full item
// metadata (label/color) survives re-renders without a re-fetch.
interface MultiProps {
  multi: true;
  selectedItems: AsyncPickerItem[];
  onChange: (items: AsyncPickerItem[]) => void;
  // Single-only props are not valid here.
  value?: never;
  selectedLabel?: never;
}

type Props = (SingleProps | MultiProps) & {
  label?: string;
  error?: string;
  fetcher: (query: string) => Promise<AsyncPickerItem[]>;
  placeholder?: string;
  emptyLabel?: string;
  disabled?: boolean;
};

// renderTreeRows walks the flat result list and, for items carrying a
// `path`, interleaves non-selectable ancestor header rows — matching GLPI's
// own dropdown: a shared ancestor (e.g. the same parent category) prints
// once, immediately followed by its children, each indented one level
// further and prefixed with a single "»" (not one "»" per depth level).
function renderTreeRows(
  items: AsyncPickerItem[],
  opts: {
    multi: boolean;
    selectedIDs: Set<number>;
    selectedValue: number | null | undefined;
    onPick: (item: AsyncPickerItem) => void;
  }
): ReactNode[] {
  const rows: ReactNode[] = [];
  // The chain of labels already visible on screen, ending with whatever the
  // previous row actually displayed (header OR selectable item) — so a
  // parent that is itself selectable (shown as its own row) doesn't get
  // reprinted as a separate header line right above its children.
  let shown: string[] = [];
  // GLPI's own tree search collapses depth to two visual tiers — the root
  // group sits flush left with no marker, and every row below it (no matter
  // how many real levels deep) gets the same single indent + single "»".
  const indent = (depth: number) => ({ paddingLeft: depth > 0 ? "26px" : "12px" });
  const marker = (depth: number) => (depth > 0 ? "»" : "");

  for (const item of items) {
    const path = item.path ?? [];
    for (let depth = 0; depth < path.length; depth++) {
      if (shown[depth] === path[depth]) continue;
      rows.push(
        <div
          key={`hdr-${item.id}-${depth}`}
          style={indent(depth)}
          className="pr-3 py-1 text-xs font-medium text-[var(--text-faint)] truncate"
        >
          {marker(depth)}
          {path[depth]}
        </div>
      );
      // Everything below this depth is now stale and must reprint too.
      shown = shown.slice(0, depth);
      shown[depth] = path[depth];
    }

    const picked = opts.multi ? opts.selectedIDs.has(item.id) : item.id === opts.selectedValue;
    rows.push(
      <button
        key={item.id}
        type="button"
        onClick={() => opts.onPick(item)}
        style={indent(path.length)}
        className={`w-full text-left pr-3 py-1.5 text-sm transition-colors flex items-start gap-2 ${
          picked
            ? "bg-[var(--accent-muted)] text-[var(--accent)]"
            : "text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
        }`}
      >
        {opts.multi && (
          <span
            className={`mt-0.5 inline-flex items-center justify-center w-4 h-4 rounded-[var(--radius-sm)] border shrink-0 ${
              picked
                ? "bg-[var(--accent)] border-[var(--accent)] text-white"
                : "border-[var(--border-default)]"
            }`}
            aria-hidden
          >
            {picked && <Icon path={ICON_PATHS.check} className="w-3 h-3" strokeWidth={3} />}
          </span>
        )}
        <span className="flex-1 min-w-0">
          <span className="flex items-center gap-1.5">
            {item.color && (
              <span className="inline-block w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: item.color }} />
            )}
            <span className="block truncate">
              {path.length > 0 ? "»" : ""}
              {item.label}
            </span>
          </span>
          {item.secondary && <span className="block text-xs text-[var(--text-faint)]">{item.secondary}</span>}
        </span>
      </button>
    );
    shown = [...path, item.label];
  }
  return rows;
}

export default function AsyncPicker(props: Props) {
  const { t } = useLocale();
  const {
    label,
    error,
    fetcher,
    placeholder,
    emptyLabel,
    disabled,
  } = props;
  const multi = props.multi === true;
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [items, setItems] = useState<AsyncPickerItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [fetchErr, setFetchErr] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const reqTokenRef = useRef(0);

  // Debounced search — new fetch on every keystroke, ignoring out-of-order results.
  useEffect(() => {
    if (!open) return;
    setLoading(true);
    const token = ++reqTokenRef.current;
    const timer = setTimeout(() => {
      fetcher(query)
        .then((rows) => {
          if (reqTokenRef.current !== token) return;
          setItems(rows);
          setFetchErr(null);
        })
        .catch((err) => {
          if (reqTokenRef.current !== token) return;
          setFetchErr(err instanceof Error ? err.message : t("common.fetchFailed"));
          setItems([]);
        })
        .finally(() => {
          if (reqTokenRef.current === token) setLoading(false);
        });
    }, 200);
    return () => clearTimeout(timer);
  }, [query, open, fetcher, t]);

  const selectedItems = multi ? props.selectedItems : [];
  const selectedIDs = new Set(selectedItems.map((i) => i.id));
  const singleLabel = !multi && props.value != null ? props.selectedLabel || `#${props.value}` : "";

  const pickSingle = (item: AsyncPickerItem | null) => {
    if (multi) return;
    (props.onChange as (i: AsyncPickerItem | null) => void)(item);
    setOpen(false);
    setQuery("");
  };

  const toggleMulti = (item: AsyncPickerItem) => {
    if (!multi) return;
    const on = props.onChange as (items: AsyncPickerItem[]) => void;
    on(
      selectedIDs.has(item.id)
        ? selectedItems.filter((s) => s.id !== item.id)
        : [...selectedItems, item]
    );
  };

  const removeMulti = (id: number) => {
    if (!multi) return;
    (props.onChange as (items: AsyncPickerItem[]) => void)(
      selectedItems.filter((s) => s.id !== id)
    );
  };

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <Popover.Root
        open={open}
        onOpenChange={(o) => {
          setOpen(o);
          if (!o) setQuery("");
        }}
      >
        <Popover.Trigger asChild disabled={disabled}>
          <button
            type="button"
            disabled={disabled}
            className={`w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border rounded-[var(--radius-md)] px-3 py-2 text-sm text-left flex items-center justify-between gap-2 disabled:opacity-40 ${
              open
                ? "border-[var(--accent)] ring-2 ring-[var(--accent-muted)]"
                : error
                ? "border-[var(--danger)]"
                : "border-[var(--border-default)]"
            }`}
          >
            {multi ? (
              <span className="flex-1 flex flex-wrap gap-1 min-w-0">
                {selectedItems.length === 0 ? (
                  <span className="text-[var(--text-faint)]">
                    {placeholder || t("common.selectPlaceholder")}
                  </span>
                ) : (
                  selectedItems.map((s) => (
                    <span
                      key={s.id}
                      className="inline-flex items-center gap-1.5 bg-[var(--bg-surface)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] pl-1.5 pr-0.5 py-0.5 text-xs"
                    >
                      {s.color && (
                        <span
                          className="inline-block w-2 h-2 rounded-full shrink-0"
                          style={{ backgroundColor: s.color }}
                        />
                      )}
                      <span className="truncate max-w-[180px]">{s.label}</span>
                      {!disabled && (
                        <button
                          type="button"
                          onClick={(e) => {
                            e.stopPropagation();
                            removeMulti(s.id);
                          }}
                          className="w-4 h-4 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-faint)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-elevated)]"
                          aria-label={t("common.removeItem", { label: s.label })}
                        >
                          ×
                        </button>
                      )}
                    </span>
                  ))
                )}
              </span>
            ) : (
              <span className={props.value != null ? "" : "text-[var(--text-faint)]"}>
                {singleLabel || placeholder || t("common.selectPlaceholder")}
              </span>
            )}
            <Icon path={ICON_PATHS.chevronDown} className={`w-3.5 h-3.5 text-[var(--text-faint)] shrink-0 transition-transform ${open ? "rotate-180" : ""}`} />
          </button>
        </Popover.Trigger>
        <Popover.Portal>
          <Popover.Content
            side="bottom"
            sideOffset={4}
            align="start"
            className="z-[100] bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg overflow-hidden animate-fade-in"
            style={{ width: "var(--radix-popover-trigger-width)" }}
            onOpenAutoFocus={(e) => {
              e.preventDefault();
              inputRef.current?.focus();
            }}
          >
            <div className="p-1.5 border-b border-[var(--border-subtle)]">
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={placeholder || t("common.search")}
                className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2.5 py-1.5 text-sm focus:outline-none focus:border-[var(--accent)]"
              />
            </div>
            <div className="max-h-64 overflow-y-auto">
              {!multi && props.value != null && (
                <button
                  type="button"
                  onClick={() => pickSingle(null)}
                  className="w-full text-left px-3 py-1.5 text-xs text-[var(--text-muted)] hover:bg-[var(--bg-elevated)]"
                >
                  {t("ui.asyncPicker.clearSelection")}
                </button>
              )}
              {loading && (
                <div className="px-3 py-2 text-xs text-[var(--text-muted)] animate-pulse">{t("common.loading")}</div>
              )}
              {fetchErr && (
                <div className="px-3 py-2 text-xs text-[var(--danger)]">{fetchErr}</div>
              )}
              {!loading && !fetchErr && items.length === 0 && (
                <div className="px-3 py-2 text-xs text-[var(--text-faint)]">
                  {emptyLabel ?? t("common.noResultsShort")}
                </div>
              )}
              {!loading && renderTreeRows(items, {
                multi,
                selectedIDs,
                selectedValue: !multi ? props.value : undefined,
                onPick: (item) => (multi ? toggleMulti(item) : pickSingle(item)),
              })}
            </div>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>
      {error && <p className="text-xs text-[var(--danger)]">{error}</p>}
    </div>
  );
}
