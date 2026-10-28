"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Input from "@/components/ui/Input";
import Tag from "@/components/ui/Tag";
import Badge from "@/components/ui/Badge";
import { gestorAPI } from "@/lib/api";
import { useDebounce } from "@/hooks/useDebounce";
import { useLocale } from "@/contexts/LocaleContext";
import type { ProjectGestorLink } from "@/lib/types";

interface Props {
  value: ProjectGestorLink[];
  onChange: (rows: ProjectGestorLink[]) => void;
}

/** Gestor projects this Bridge project refers to: search by name, pick, remove. Reference only. */
export default function GestorLinksEditor({ value, onChange }: Props) {
  const { t } = useLocale();
  const [q, setQ] = useState("");
  const dq = useDebounce(q.trim(), 300);
  const { data, isError, isFetching } = useQuery({ queryKey: ["gestor-projetos", dq], queryFn: () => gestorAPI.projetos(dq), staleTime: 60_000, retry: false });

  if (isError || data?.error || data?.enabled === false) {
    return <p className="text-xs text-[var(--text-muted)]">{t("project.gestorUnavailable")}</p>;
  }
  const picked = new Set(value.map((l) => l.gestor_projeto_id));
  const results = (dq ? data?.data ?? [] : []).filter((p) => !picked.has(p.id));
  const stateOf = (c?: boolean, a?: boolean) => (a ? t("project.gestorArquivado") : c ? t("project.gestorConcluido") : "");

  return (
    <div className="space-y-2">
      {value.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {value.map((l) => (
            <span key={l.gestor_projeto_id} className="inline-flex items-center gap-1">
              <Tag removeLabel={t("common.removeItem", { label: l.gestor_projeto_nome })}
                onRemove={() => onChange(value.filter((x) => x.gestor_projeto_id !== l.gestor_projeto_id))}>
                {l.gestor_projeto_nome || `#${l.gestor_projeto_id}`}
              </Tag>
              {stateOf(l.concluido, l.arquivado) && <Badge color="warning">{stateOf(l.concluido, l.arquivado)}</Badge>}
            </span>
          ))}
        </div>
      )}
      <Input aria-label={t("project.gestorSearch")} placeholder={t("project.gestorSearch")} value={q} onChange={(e) => setQ(e.target.value)} />
      {dq && !isFetching && results.length === 0 && <p className="text-xs text-[var(--text-muted)]">{t("project.gestorNoResults")}</p>}
      {results.length > 0 && (
        <ul className="rounded-[var(--radius-md)] border border-[var(--border-default)] divide-y divide-[var(--border-default)]">
          {results.map((p) => (
            <li key={p.id}>
              <button type="button" className="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm hover:bg-[var(--bg-overlay)]"
                onClick={() => {
                  onChange([...value, { gestor_projeto_id: p.id, gestor_projeto_nome: p.nome, concluido: p.concluido, arquivado: p.arquivado }]);
                  setQ("");
                }}>
                <span className="truncate">{p.nome}</span>
                {stateOf(p.concluido, p.arquivado) && <Badge color="warning">{stateOf(p.concluido, p.arquivado)}</Badge>}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
