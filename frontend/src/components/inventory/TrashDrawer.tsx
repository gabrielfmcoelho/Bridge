"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient, type QueryKey } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { useFlag } from "@/contexts/FlagContext";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import EmptyState from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import ToolbarActionButton from "@/components/ui/ToolbarActionButton";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";

type Trashed = { id: number; deleted_at?: string | null };

export interface TrashSource<T extends Trashed> {
  /** Cache key of the trash list, e.g. ["hosts-trash"]. */
  key: QueryKey;
  fetch: () => Promise<T[]>;
  restore: (id: number) => Promise<unknown>;
  /** Live lists to refresh after a restore. */
  invalidate: QueryKey[];
  label: (item: T) => string;
  meta?: (item: T) => string | undefined;
}

/**
 * A module's trash in its own page: a toolbar button ("Lixeira · N") that
 * opens a drawer listing what was deleted there, each restorable. Restoring
 * is an admin action, so the button only shows for admins (unless the
 * module's trash is already scoped to what the caller may restore).
 */
export default function TrashButton<T extends Trashed>({ title, source, everyone }: {
  title: string;
  source: TrashSource<T>;
  /** The trash lists only what the caller may restore (the vault's own secrets). */
  everyone?: boolean;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const [open, setOpen] = useState(false);
  const isAdmin = everyone || user?.role === "admin";
  // The count shows on the button, so the list loads with the page (it's small).
  const { data: items = [], isLoading } = useQuery({ queryKey: source.key, queryFn: source.fetch, enabled: isAdmin });
  if (!isAdmin) return null;
  return (
    <>
      <ToolbarActionButton icon={ICON_PATHS.trashOutline} hideLabel="md" onClick={() => setOpen(true)}
        label={items.length ? t("trash.buttonCount", { count: String(items.length) }) : t("trash.button")} />
      <TrashDrawer open={open} onClose={() => setOpen(false)} title={title} source={source} items={items} loading={isLoading} />
    </>
  );
}

function TrashDrawer<T extends Trashed>({ open, onClose, title, source, items, loading }: {
  open: boolean;
  onClose: () => void;
  title: string;
  source: TrashSource<T>;
  items: T[];
  loading: boolean;
}) {
  const { t, locale } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const restore = useMutation({
    mutationFn: (item: T) => source.restore(item.id),
    onSuccess: (_, item) => {
      qc.invalidateQueries({ queryKey: source.key });
      source.invalidate.forEach((k) => qc.invalidateQueries({ queryKey: k }));
      flag({ appearance: "success", title: t("trash.restoredTitle"), description: source.label(item) });
    },
    onError: (e) => flag({ appearance: "error", title: t("trash.restore"), description: e instanceof Error ? e.message : t("form.saveFailed") }),
  });

  return (
    <Drawer open={open} onClose={onClose} title={title}>
      <p className="text-sm text-[var(--text-secondary)] mb-4">{t("trash.drawerHint")}</p>
      {loading ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <Skeleton key={i} className="h-12 w-full" />)}</div>
      ) : items.length === 0 ? (
        <EmptyState compact icon="trash" title={t("trash.emptyTitle")} description={t("trash.emptyModule")} />
      ) : (
        <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] overflow-hidden">
          <RowList>
            {items.map((item) => (
              <ListRow key={item.id}>
                <RowText title={source.label(item)} meta={<>
                  {source.meta?.(item) && <span>{source.meta(item)}</span>}
                  {item.deleted_at && <span>{t("trash.deletedAgo", { ago: getTimeAgo(item.deleted_at, locale) })}</span>}
                </>} />
                <Button size="sm" variant="secondary" loading={restore.isPending && restore.variables?.id === item.id}
                  onClick={() => restore.mutate(item)}>{t("trash.restore")}</Button>
              </ListRow>
            ))}
          </RowList>
        </div>
      )}
    </Drawer>
  );
}
