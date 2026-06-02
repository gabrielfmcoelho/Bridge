"use client";

import { useState } from "react";
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import { secretsAPI } from "@/lib/api";
import { useSecretReveal } from "@/hooks/useSecretReveal";
import { useLocale } from "@/contexts/LocaleContext";
import type { Secret } from "@/lib/types";

// parseFields turns a revealed payload into copyable fields. Most payloads are
// JSON ({value}, {username,password}, …) — we surface each field on its own
// rather than dumping raw JSON. A lone `value` field is shown label-less (it
// IS the secret); other keys keep their label so multi-field secrets stay
// usable. Non-JSON payloads become a single unlabeled field.
function parseFields(raw: string): { label: string; value: string }[] {
  const t = raw.trim();
  if (t.startsWith("{")) {
    try {
      const obj = JSON.parse(t);
      if (obj && typeof obj === "object" && !Array.isArray(obj)) {
        const entries = Object.entries(obj).filter(([, v]) => v != null && v !== "");
        if (entries.length > 0) {
          return entries.map(([k, v]) => ({ label: k === "value" ? "" : k, value: String(v) }));
        }
      }
    } catch {
      /* not JSON — fall through */
    }
  }
  return [{ label: "", value: raw }];
}

// CopyField renders one revealed value as a read-only input (textarea for
// multi-line values like PEM keys) with its own copy button. Copy reads the
// underlying string, never the DOM, so it's correct even when an <input>
// visually clips newlines.
function CopyField({ label, value }: { label: string; value: string }) {
  const { t } = useLocale();
  const [copied, setCopied] = useState(false);
  const multiline = value.includes("\n") || value.length > 120;

  async function copy() {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <div>
      {label && (
        <label className="block text-[10px] font-semibold uppercase tracking-wide text-[var(--text-muted)] mb-1">
          {label}
        </label>
      )}
      <div className="flex gap-1.5 items-start">
        {multiline ? (
          <Textarea readOnly value={value} rows={4} className="flex-1 font-mono text-xs" />
        ) : (
          <Input readOnly value={value} className="flex-1 font-mono text-xs" />
        )}
        <Button variant="secondary" size="sm" type="button" onClick={copy}>
          {copied ? t("atlas.apis.copied") : t("atlas.apis.copyValue")}
        </Button>
      </div>
    </div>
  );
}

// One row per secret with its own reveal lifecycle (the hook is single-tenant
// per instance, so each row gets its own 30s auto-hide timer + clipboard).
function SecretRow({ secret }: { secret: Secret }) {
  const { t } = useLocale();
  const r = useSecretReveal();

  return (
    <div className="border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-3">
      <div className="flex items-center justify-between gap-2 mb-2">
        <div className="min-w-0">
          <span className="text-sm font-medium text-[var(--text-primary)] truncate">{secret.name}</span>
          <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded border border-[var(--border-default)] bg-[var(--bg-overlay)] text-[var(--text-muted)]">
            {secret.type}
          </span>
        </div>
        {!r.revealed ? (
          <Button variant="secondary" size="sm" type="button" loading={r.loading} onClick={() => r.reveal(secret.id)}>
            {t("atlas.apis.reveal")}
          </Button>
        ) : (
          <Button variant="ghost" size="sm" type="button" onClick={() => r.hide()}>
            {t("atlas.apis.hide")}
          </Button>
        )}
      </div>
      {secret.description && <p className="text-xs text-[var(--text-muted)] mb-2">{secret.description}</p>}
      {r.error && <p className="text-xs text-red-400">{r.error}</p>}
      {r.revealed && r.value != null && (
        <div className="space-y-2">
          {parseFields(r.value).map((f, i) => (
            <CopyField key={i} label={f.label} value={f.value} />
          ))}
        </div>
      )}
    </div>
  );
}

// ProjectSecretsSheet is a right-hand drawer listing the secrets attached to
// the API's project, each revealable + copyable in place (requirement #7).
export default function ProjectSecretsSheet({
  open,
  onClose,
  projectId,
}: {
  open: boolean;
  onClose: () => void;
  projectId: number;
}) {
  const { t } = useLocale();
  const { data: secrets = [], isLoading } = useQuery({
    queryKey: ["secrets", "projeto", projectId],
    queryFn: () => secretsAPI.list({ scope: "projeto", parent_id: projectId }),
    enabled: open,
  });

  return (
    <Drawer open={open} onClose={onClose} title={t("atlas.apis.projectSecrets")} side="right">
      <div className="p-4 space-y-3">
        <Link
          href={`/secrets?scope=projeto&parent_id=${projectId}`}
          className="text-xs text-[var(--accent)] hover:underline"
        >
          {t("atlas.apis.viewAllSecrets")} →
        </Link>
        {isLoading ? (
          <div className="h-10 skeleton rounded-[var(--radius-md)]" />
        ) : secrets.length === 0 ? (
          <p className="text-sm text-[var(--text-muted)]">{t("atlas.apis.noProjectSecrets")}</p>
        ) : (
          secrets.map((s) => <SecretRow key={s.id} secret={s} />)
        )}
      </div>
    </Drawer>
  );
}
