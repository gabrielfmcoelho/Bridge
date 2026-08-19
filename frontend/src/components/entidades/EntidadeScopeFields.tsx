"use client";

import { useEffect, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { assetEntidadesAPI, entidadesAPI } from "@/lib/api";
import { creatorOptions, indentedLabel, withDepth } from "@/lib/entidades";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import type { AssetGrantsInput, AssetType } from "@/lib/types";
import Select from "@/components/ui/Select";
import CheckboxList from "@/components/ui/CheckboxList";
import Toggle from "@/components/ui/Toggle";

/**
 * The one place every asset form picks "who sees this": creator entidade
 * (defaults to the user's primary), responsible entidades, and the global
 * flag. Controlled; the parent owns the AssetGrantsInput and spreads it into
 * the create/update payload.
 */
export default function EntidadeScopeFields({
  value,
  onChange,
  compact = false,
  loadFrom,
}: {
  value: AssetGrantsInput;
  onChange: (v: AssetGrantsInput) => void;
  compact?: boolean;
  /** Edit forms without the grants in hand: fetch them once and push into value. */
  loadFrom?: { type: AssetType; id: number } | null;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const { data: entidades = [] } = useQuery({ queryKey: ["entidades"], queryFn: entidadesAPI.list });
  const { data: loaded } = useQuery({
    queryKey: ["asset-grants", loadFrom?.type, loadFrom?.id],
    queryFn: () => assetEntidadesAPI.get(loadFrom!.type, loadFrom!.id),
    enabled: !!loadFrom,
  });
  const pushed = useRef(false);
  useEffect(() => {
    if (loaded && !pushed.current) {
      pushed.current = true;
      onChange(loaded);
    }
  }, [loaded, onChange]);

  const creators = creatorOptions(entidades, user);
  const all = withDepth(entidades);

  return (
    <div className={compact ? "space-y-3" : "space-y-4"}>
      <Select
        label={t("entidades.creator")}
        value={value.creator_entidade_id != null ? String(value.creator_entidade_id) : ""}
        onChange={(e) => onChange({ ...value, creator_entidade_id: e.target.value ? Number(e.target.value) : null })}
        options={[{ value: "", label: t("entidades.none") }, ...creators.map((e) => ({ value: String(e.id), label: indentedLabel(e) }))]}
      />
      <CheckboxList
        label={t("entidades.responsibles")}
        items={all.map((e) => ({ id: e.id, name: indentedLabel(e) }))}
        selected={value.responsible_entidade_ids ?? []}
        onChange={(ids) => onChange({ ...value, responsible_entidade_ids: ids })}
      />
      <div className="flex items-center justify-between">
        <div>
          <div className="text-xs font-medium text-[var(--text-secondary)]">{t("entidades.global")}</div>
          <div className="text-[11px] text-[var(--text-faint)]">{t("entidades.globalHint")}</div>
        </div>
        <Toggle checked={!!value.is_global} onChange={(v) => onChange({ ...value, is_global: v })} ariaLabel={t("entidades.global")} />
      </div>
    </div>
  );
}

/** Default grants for a brand-new asset: creator = the user's primary entidade. */
export function defaultGrants(user: { entidades?: { id: number; is_primary: boolean }[] } | null): AssetGrantsInput {
  const primary = user?.entidades?.find((e) => e.is_primary) ?? user?.entidades?.[0];
  return { creator_entidade_id: primary?.id ?? null, responsible_entidade_ids: [], is_global: false };
}
