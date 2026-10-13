"use client";

import { useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { apiKeysAPI } from "@/lib/api";
import type { ApiCatalog, ApiKey, ApiKeyTestResult } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import Modal from "@/components/ui/Modal";
import Input from "@/components/ui/Input";
import Button from "@/components/ui/Button";
import Lozenge from "@/components/ui/Lozenge";
import StatusAlert from "@/components/ui/StatusAlert";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";

// Key test: Bridge gets a token of the key's client from Keycloak and calls
// the API through its base URL (the gateway). The token never reaches here.

type T = ReturnType<typeof useLocale>["t"];

// diagnosis turns a failed verdict into the likely cause (null: nothing to add).
function diagnosis(r: ApiKeyTestResult, t: T): string | null {
  if (!r.token_ok || r.error) return null;
  if (r.missing.length > 0) return t("atlas.apis.keys.test.hintMissing");
  if (r.status === 401) return t(r.answered_by === "gateway" ? "atlas.apis.keys.test.hintGateway401" : "atlas.apis.keys.test.hintApi401");
  if (r.status === 403) return t("atlas.apis.keys.test.hintApi403");
  if (r.ok && r.status === 404) return t("atlas.apis.keys.test.hintPassed");
  return null;
}

function answer(r: ApiKeyTestResult, t: T) {
  if (r.error) return `${t("atlas.apis.keys.test.callError")}: ${r.error}`;
  return `${r.status} ${t(r.answered_by === "gateway" ? "atlas.apis.keys.test.byGateway" : "atlas.apis.keys.test.byApi")}`;
}

function Verdict({ ok }: { ok: boolean }) {
  const { t } = useLocale();
  return <Lozenge appearance={ok ? "success" : "removed"}>{t(ok ? "atlas.apis.keys.test.ok" : "atlas.apis.keys.test.failed")}</Lozenge>;
}

function Result({ r }: { r: ApiKeyTestResult }) {
  const { t } = useLocale();
  const hint = diagnosis(r, t);
  const rows: [string, string][] = [
    [t("atlas.apis.keys.test.token"), r.token_ok
      ? [t("atlas.apis.keys.test.tokenOk"), r.expires_in ? t("atlas.apis.keys.test.expiresIn", { seconds: String(r.expires_in) }) : ""].filter(Boolean).join(", ")
      : r.token_error ?? ""],
  ];
  if (r.token_ok) {
    rows.push([t("atlas.apis.keys.test.granted"), r.granted.join(", ") || "—"]);
    if (r.missing.length > 0) rows.push([t("atlas.apis.keys.test.missing"), r.missing.join(", ")]);
    if (r.rate_limit_per_minute) rows.push([t("atlas.apis.keys.test.rateLimit"), String(r.rate_limit_per_minute)]);
    rows.push([t("atlas.apis.keys.test.answer"), answer(r, t)]);
  }
  rows.push([t("atlas.apis.keys.test.address"), r.url]);
  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Verdict ok={r.ok} />
        <span className="font-mono text-xs text-[var(--text-secondary)]">{r.client_id}</span>
      </div>
      {hint && <StatusAlert variant={r.ok ? "info" : "warning"}>{hint}</StatusAlert>}
      <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
        {rows.map(([k, v]) => (
          <div key={k} className="contents">
            <dt className="text-[var(--text-muted)]">{k}</dt>
            <dd className="font-mono text-[var(--text-primary)] break-all">{v}</dd>
          </div>
        ))}
      </dl>
      {r.body && (
        <pre className="text-2xs font-mono whitespace-pre-wrap break-all px-3 py-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)] text-[var(--text-secondary)]">
          {r.body}
        </pre>
      )}
    </div>
  );
}

// KeyTestModal tests one key, on the default path or a real GET route.
export function KeyTestModal({ api, apiKey, onClose }: { api: ApiCatalog; apiKey: ApiKey; onClose: () => void }) {
  const { t } = useLocale();
  const [path, setPath] = useState("");
  const run = useMutation({ mutationFn: () => apiKeysAPI.test(api.id, apiKey.id, path.trim() || undefined) });
  return (
    <Modal open onClose={onClose} title={t("atlas.apis.keys.test.title", { name: apiKey.label })}>
      <div className="space-y-4">
        <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.keys.test.intro")}</p>
        <form className="flex flex-col sm:flex-row gap-2 sm:items-start" onSubmit={(e) => { e.preventDefault(); run.mutate(); }}>
          <div className="flex-1 min-w-0">
            <Input className="font-mono" label={t("atlas.apis.keys.test.path")} hint={t("atlas.apis.keys.test.pathHint")}
              value={path} onChange={(e) => setPath(e.target.value)} placeholder="/api/…" />
          </div>
          <Button type="submit" className="sm:mt-6" disabled={run.isPending}>
            {t(run.isPending ? "atlas.apis.keys.test.running" : "atlas.apis.keys.test.run")}
          </Button>
        </form>
        {!!run.error && <StatusAlert variant="error">{(run.error as Error).message}</StatusAlert>}
        {run.data && <Result r={run.data} />}
      </div>
    </Modal>
  );
}

// KeyTestAllModal runs the default test on every active key as it opens.
export function KeyTestAllModal({ api, onClose }: { api: ApiCatalog; onClose: () => void }) {
  const { t } = useLocale();
  const { data, error, isFetching } = useQuery({
    queryKey: ["api-keys", api.id, "test-all"],
    queryFn: () => apiKeysAPI.testAll(api.id),
    retry: false,
    gcTime: 0,
  });
  return (
    <Modal open onClose={onClose} size="lg" title={t("atlas.apis.keys.test.allTitle")}>
      <div className="space-y-3">
        <p className="text-sm text-[var(--text-secondary)]">{t("atlas.apis.keys.test.batchHint")}</p>
        {isFetching && <p className="text-sm text-[var(--text-muted)]">{t("atlas.apis.keys.test.running")}</p>}
        {!!error && <StatusAlert variant="error">{(error as Error).message}</StatusAlert>}
        {data && !isFetching && (
          data.total === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">{t("atlas.apis.keys.test.none")}</p>
          ) : (
            <>
              <StatusAlert variant={data.ok === data.total ? "success" : "warning"}>
                {t("atlas.apis.keys.test.summary", { ok: String(data.ok), total: String(data.total) })}
              </StatusAlert>
              <RowList>
                {data.results.map((r) => {
                  const meta = [
                    r.client_id,
                    r.token_ok ? answer(r, t) : r.token_error ?? "",
                    ...(r.missing.length > 0 ? [`${t("atlas.apis.keys.test.missing")}: ${r.missing.join(", ")}`] : []),
                    ...(!r.ok ? [diagnosis(r, t) ?? ""].filter(Boolean) : []),
                  ];
                  return (
                    <ListRow key={r.key_id}>
                      <RowText title={r.label} mono meta={meta.map((m, i) => <span key={i}>{m}</span>)} />
                      <Verdict ok={r.ok} />
                    </ListRow>
                  );
                })}
              </RowList>
            </>
          )
        )}
      </div>
    </Modal>
  );
}
