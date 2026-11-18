"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Button from "@/components/ui/Button";
import Select from "@/components/ui/Select";
import NativeSelect from "@/components/ui/NativeSelect";
import FormError from "@/components/ui/FormError";
import Lozenge from "@/components/ui/Lozenge";
import { RowGroup, ListRow } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import { appDepsAPI } from "@/lib/api";
import { APP_LINK_TYPES, isManual, linkSourceKey } from "@/lib/appDeps";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppDepLinks } from "@/hooks/useAppDepLinks";
import type { AppDepAssetLink, AppDepEntityType } from "@/lib/types";
import { useAppDepAssets } from "./useAppDepAssets";

type Target = { ci: string } | { entity: { type: AppDepEntityType; id: number } };

/** Opens the manager of an application's links (from its page) or of an
 *  asset's application links (from a host, DNS, service or projeto page).
 *  Only links added by hand can be removed here; derived ones are listed
 *  with where they come from. */
export default function AppDepLinksButton(props: Target) {
  const { t } = useLocale();
  const [open, setOpen] = useState(false);
  return (
    <>
      <IconButton label={t("appDeps.manage.open")} onClick={() => setOpen(true)}>
        <Icon path={ICON_PATHS.link} />
      </IconButton>
      {open && <AppDepLinksModal {...props} onClose={() => setOpen(false)} />}
    </>
  );
}

function AppDepLinksModal(props: Target & { onClose: () => void }) {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const ciMode = "ci" in props;
  const filter = ciMode ? { ci: props.ci } : { entity_type: props.entity.type, entity_id: props.entity.id };
  const { links, loading } = useAppDepLinks(filter);
  const assets = useAppDepAssets(ciMode);
  const { data: sources } = useQuery({ queryKey: ["app-deps"], queryFn: appDepsAPI.list, enabled: !ciMode });

  const [type, setType] = useState<AppDepEntityType>("host");
  const [pick, setPick] = useState("");
  const [error, setError] = useState("");

  const invalidate = () => {
    for (const key of [["app-dep-links"], ["relations"], ["graph"]]) queryClient.invalidateQueries({ queryKey: key });
  };
  const fail = (err: unknown) => setError(err instanceof Error ? err.message : t("appDeps.manage.error"));
  const add = useMutation({
    mutationFn: () => appDepsAPI.addLink(ciMode
      ? { ci: props.ci, entity_type: type, entity_id: Number(pick) }
      : { ci: pick, entity_type: props.entity.type, entity_id: props.entity.id }),
    onSuccess: () => { setPick(""); setError(""); invalidate(); },
    onError: fail,
  });
  const remove = useMutation({
    mutationFn: (l: AppDepAssetLink) => appDepsAPI.removeLink(l.ci, l.entity_type, l.entity_id),
    onSuccess: () => { setError(""); invalidate(); },
    onError: fail,
  });

  const nameOf = (l: AppDepAssetLink) => {
    if (!ciMode) return l.ci;
    return assets.byType[l.entity_type].find((a) => a.id === l.entity_id)?.label ?? `#${l.entity_id}`;
  };
  const manualKeys = new Set(links.filter(isManual).map((l) => (ciMode ? `${l.entity_type}:${l.entity_id}` : l.ci)));
  const options = useMemo(() => {
    if (ciMode) {
      return assets.byType[type].filter((a) => !manualKeys.has(`${type}:${a.id}`))
        .map((a) => ({ value: String(a.id), label: a.label }));
    }
    const cis = [...new Set((sources ?? []).map((s) => s.catalog?.ci).filter((c): c is string => !!c))].sort();
    return cis.filter((c) => !manualKeys.has(c)).map((c) => ({ value: c, label: c }));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- manualKeys derives from links
  }, [ciMode, assets.byType, type, sources, links]);

  const groups = ciMode
    ? APP_LINK_TYPES.map((tp) => ({ title: t(`appDeps.entity.${tp}`), rows: links.filter((l) => l.entity_type === tp) }))
    : [{ title: t("appDeps.relations.apps"), rows: links }];

  return (
    <Modal open onClose={props.onClose} size="md" title={t("appDeps.manage.title")}>
      <div className="space-y-5">
        <p className="text-sm text-[var(--text-muted)]">{t(ciMode ? "appDeps.manage.hintCi" : "appDeps.manage.hintEntity")}</p>
        {error && <FormError message={error} />}

        <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] overflow-hidden">
          {loading || (ciMode && assets.loading) ? (
            <p className="px-4 py-3 text-sm text-[var(--text-muted)]">{t("common.loading")}</p>
          ) : links.length === 0 ? (
            <p className="px-4 py-3 text-sm text-[var(--text-muted)]">{t("appDeps.manage.none")}</p>
          ) : (
            groups.filter((g) => g.rows.length > 0).map((g) => (
              <RowGroup key={g.title} title={g.title}>
                {g.rows.map((l) => (
                  <ListRow key={`${l.ci}|${l.entity_type}|${l.entity_id}`}>
                    <span className="text-sm font-mono truncate flex-1 text-[var(--text-primary)]">{nameOf(l)}</span>
                    <Lozenge>{t(linkSourceKey(l))}</Lozenge>
                    {isManual(l) ? (
                      <IconButton label={t("appDeps.manage.remove", { name: nameOf(l) })} onClick={() => remove.mutate(l)} disabled={remove.isPending}
                        className="hover:text-[var(--danger)]">
                        <Icon path={ICON_PATHS.trash} />
                      </IconButton>
                    ) : (
                      <span className="text-2xs text-[var(--text-muted)] truncate max-w-48" title={t("appDeps.manage.derivedHint")}>{l.via}</span>
                    )}
                  </ListRow>
                ))}
              </RowGroup>
            ))
          )}
        </div>

        <div className="flex flex-wrap items-end gap-2">
          {ciMode && (
            <NativeSelect label={t("appDeps.manage.type")} value={type} onChange={(e) => { setType(e.target.value as AppDepEntityType); setPick(""); }}>
              {APP_LINK_TYPES.map((tp) => <option key={tp} value={tp}>{t(`appDeps.entity.${tp}`)}</option>)}
            </NativeSelect>
          )}
          <div className="flex-1 min-w-56">
            <Select
              label={ciMode ? t(`appDeps.entity.${type}`) : t("appDeps.manage.application")}
              value={pick}
              onChange={(e) => setPick(e.target.value)}
              options={options}
            />
          </div>
          <Button size="sm" onClick={() => add.mutate()} disabled={!pick} loading={add.isPending}>
            {t("appDeps.manage.add")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
