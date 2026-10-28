"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { gestorAPI } from "@/lib/api";
import type { GitLabAutor, GitLabAutorUpdate } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import { useFlag } from "@/contexts/FlagContext";
import { useDebounce } from "@/hooks/useDebounce";
import SectionCard from "@/components/ui/SectionCard";
import { tableClasses } from "@/components/ui/Table";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Lozenge from "@/components/ui/Lozenge";
import Modal from "@/components/ui/Modal";
import NativeSelect from "@/components/ui/NativeSelect";
import StatusAlert from "@/components/ui/StatusAlert";

const FILTERS = ["sem_vinculo", "com_sugestao", "vinculados", "ignorados", "todos"] as const;

/** Settings → Autores do GitLab: link each commit e-mail to a Gestor user. */
export default function GestorAutoresPage() {
  const { t } = useLocale();
  const confirm = useConfirm();
  const flag = useFlag();
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<(typeof FILTERS)[number]>("sem_vinculo");
  const [q, setQ] = useState("");
  const dq = useDebounce(q.trim());
  const [picking, setPicking] = useState<GitLabAutor | null>(null);

  const { data: autores = [], isLoading, error } = useQuery({
    queryKey: ["gestor-autores", status, dq],
    queryFn: () => gestorAPI.autores(status, dq),
  });
  // An empty search only tells whether the integration is on.
  const { data: pessoas } = useQuery({ queryKey: ["gestor-pessoas", ""], queryFn: () => gestorAPI.pessoas("") });

  const update = useMutation({
    mutationFn: ({ id, body }: { id: number; body: GitLabAutorUpdate }) => gestorAPI.updateAutor(id, body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["gestor-autores"] }),
    onError: (err) => flag({ appearance: "error", title: t("settings.gestorAutores.updateError"), description: err instanceof Error ? err.message : undefined }),
  });

  // The scan runs in the background: POST starts it, the status query polls
  // while it runs and the outcome flag shows when it ends.
  const { data: varredura } = useQuery({
    queryKey: ["gestor-varredura"],
    queryFn: gestorAPI.varredura,
    refetchInterval: (query) => (query.state.data?.running ? 3000 : false),
  });
  const scan = useMutation({
    mutationFn: gestorAPI.varrer,
    onSuccess: (st) => queryClient.setQueryData(["gestor-varredura"], st),
    onError: (err) => flag({ appearance: "error", title: t("settings.gestorAutores.scanError"), description: err instanceof Error ? err.message : undefined }),
  });
  const running = scan.isPending || !!varredura?.running;
  const wasRunning = useRef(false);
  useEffect(() => {
    if (!varredura) return;
    if (wasRunning.current && !varredura.running) {
      queryClient.invalidateQueries({ queryKey: ["gestor-autores"] });
      const r = varredura.result;
      if (varredura.error || !r) {
        flag({ appearance: "error", title: t("settings.gestorAutores.scanError"), description: varredura.error });
      } else {
        const erros = r.erros ?? [];
        flag({
          appearance: erros.length ? "warning" : "success",
          title: t("settings.gestorAutores.scanDone", { repos: String(r.repos), novos: String(r.autores_novos), atualizados: String(r.autores_atualizados), sugestoes: String(r.sugestoes) }),
          description: erros.length ? t("settings.gestorAutores.scanErrors", { erros: erros.slice(0, 3).join("; ") + (erros.length > 3 ? ` (+${erros.length - 3})` : "") }) : undefined,
        });
      }
    }
    wasRunning.current = varredura.running;
  }, [varredura, flag, queryClient, t]);

  const unlink = async (a: GitLabAutor) => {
    const ok = await confirm({
      title: t("settings.gestorAutores.unlinkConfirm", { email: a.email }),
      message: t("settings.gestorAutores.unlinkConfirmBody"),
      danger: true,
      confirmLabel: t("settings.gestorAutores.unlink"),
    });
    if (ok) update.mutate({ id: a.id, body: { gestor_pessoa_id: null } });
  };

  const th = tableClasses.compact.th;
  const td = tableClasses.compact.td;
  return (
    <SectionCard
      title={t("settings.gestorAutores.title")}
      description={t("settings.gestorAutores.description")}
      count={autores.length}
      controls={<Button size="sm" loading={running} disabled={running} onClick={() => scan.mutate()}>{running ? t("settings.gestorAutores.scanning") : t("settings.gestorAutores.scan")}</Button>}
    >
      {pessoas && !pessoas.enabled && <div className="mb-4"><StatusAlert variant="warning">{t("settings.gestorAutores.off")}</StatusAlert></div>}
      <div className="flex flex-wrap gap-3 mb-4">
        <NativeSelect aria-label={t("settings.gestorAutores.filterLabel")} value={status} onChange={(e) => setStatus(e.target.value as (typeof FILTERS)[number])}>
          {FILTERS.map((f) => <option key={f} value={f}>{t(`settings.gestorAutores.filter.${f}`)}</option>)}
        </NativeSelect>
        <Input className="min-w-64" placeholder={t("settings.gestorAutores.search")} value={q} onChange={(e) => setQ(e.target.value)} />
      </div>

      {error ? (
        <StatusAlert variant="error">{(error as Error).message || t("settings.gestorAutores.loadError")}</StatusAlert>
      ) : isLoading ? (
        <div className="h-10 animate-pulse bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" />
      ) : autores.length === 0 ? (
        <div className="text-center py-6 text-sm text-[var(--text-muted)]">{t("settings.gestorAutores.empty")}</div>
      ) : (
        <div className="overflow-x-auto">
          <table className={tableClasses.compact.table}>
            <thead>
              <tr className={tableClasses.compact.headRow}>
                <th className={th}>{t("settings.gestorAutores.email")}</th>
                <th className={th}>{t("settings.gestorAutores.gitlabName")}</th>
                <th className={th}>{t("settings.gestorAutores.commits")}</th>
                <th className={th}>{t("settings.gestorAutores.repos")}</th>
                <th className={th}>{t("settings.gestorAutores.link")}</th>
                <th className={th} />
              </tr>
            </thead>
            <tbody>
              {autores.map((a) => (
                <tr key={a.id} className={tableClasses.compact.row}>
                  <td className={`${td} font-mono text-xs`}>{a.email}</td>
                  <td className={td}>{a.nome_gitlab}</td>
                  <td className={td}>{a.commits}</td>
                  <td className={td}>{a.repos}</td>
                  <td className={td}>
                    {a.gestor_pessoa_id != null ? (
                      <span>{a.gestor_nome}{a.gestor_email && <span className="block text-2xs text-[var(--text-muted)]">{a.gestor_email}</span>}</span>
                    ) : a.sugestao_pessoa_id != null ? (
                      <span className="text-[var(--text-secondary)]">
                        {t("settings.gestorAutores.suggestion", { nome: a.sugestao_nome })}
                        {a.sugestao_motivo && <span className="ml-2"><Lozenge appearance="inprogress">{t(`settings.gestorAutores.reason.${a.sugestao_motivo}`)}</Lozenge></span>}
                      </span>
                    ) : null}
                    {a.ignorado && <span className="ml-2"><Lozenge>{t("settings.gestorAutores.ignored")}</Lozenge></span>}
                  </td>
                  <td className={`${td} whitespace-nowrap`}>
                    <div className="flex gap-1 justify-end">
                      {a.gestor_pessoa_id == null && a.sugestao_pessoa_id != null && !a.ignorado && (
                        <Button size="sm" onClick={() => update.mutate({ id: a.id, body: { gestor_pessoa_id: a.sugestao_pessoa_id, ...(a.sugestao_motivo === "email" ? { gestor_email: a.email } : {}) } })}>{t("settings.gestorAutores.accept")}</Button>
                      )}
                      {!a.ignorado && <Button size="sm" variant="secondary" onClick={() => setPicking(a)}>{t("settings.gestorAutores.choose")}</Button>}
                      {a.gestor_pessoa_id != null && <Button size="sm" variant="danger" onClick={() => unlink(a)}>{t("settings.gestorAutores.unlink")}</Button>}
                      <Button size="sm" variant="ghost" onClick={() => update.mutate({ id: a.id, body: { ignorado: !a.ignorado } })}>
                        {a.ignorado ? t("settings.gestorAutores.reactivate") : t("settings.gestorAutores.ignore")}
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {picking && (
        <PessoaPicker
          autor={picking}
          onClose={() => setPicking(null)}
          onPick={(p) => {
            update.mutate({ id: picking.id, body: { gestor_pessoa_id: p.id, gestor_nome: p.nome, gestor_email: p.email } });
            setPicking(null);
          }}
        />
      )}
    </SectionCard>
  );
}

function PessoaPicker({ autor, onClose, onPick }: { autor: GitLabAutor; onClose: () => void; onPick: (p: { id: number; nome: string; email: string }) => void }) {
  const { t } = useLocale();
  const [q, setQ] = useState(autor.nome_gitlab);
  const dq = useDebounce(q.trim());
  const { data, isFetching } = useQuery({ queryKey: ["gestor-pessoas", dq], queryFn: () => gestorAPI.pessoas(dq), enabled: dq.length >= 2 });
  return (
    <Modal open onClose={onClose} title={t("settings.gestorAutores.pickTitle", { email: autor.email })}>
      <Input autoFocus label={t("settings.gestorAutores.pickSearch")} value={q} onChange={(e) => setQ(e.target.value)} />
      {data?.error && <div className="mt-3"><StatusAlert variant="error">{t("settings.gestorAutores.pickError")}</StatusAlert></div>}
      <ul className="mt-3 divide-y divide-[var(--border-subtle)]">
        {(data?.data ?? []).map((p) => (
          <li key={p.id}>
            <button type="button" className="w-full text-left px-2 py-2 hover:bg-[var(--bg-elevated)] rounded-[var(--radius-md)]" onClick={() => onPick(p)}>
              <span className="text-sm">{p.nome}</span>
              <span className="block text-2xs text-[var(--text-muted)]">{[p.email, p.lotacao].filter(Boolean).join(" · ")}</span>
            </button>
          </li>
        ))}
      </ul>
      {data && !data.error && !isFetching && data.data.length === 0 && <p className="mt-3 text-sm text-[var(--text-muted)]">{t("settings.gestorAutores.pickEmpty")}</p>}
    </Modal>
  );
}
