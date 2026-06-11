"use client";

import { use, useCallback, useEffect, useState } from "react";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Card from "@/components/ui/Card";
import ApiReference from "@/components/atlas/apis/ApiReference";
import type { BundlePayload } from "@/lib/types";
import { useTheme } from "@/contexts/ThemeContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import { useLocale } from "@/contexts/LocaleContext";

// Public reveal page. Reached for BOTH kinds of share link:
//   - a single secret  (ShareLinkModal → GET /api/share/{token})
//   - a heterogeneous bundle (ShareBundleModal → GET /api/share-bundle/{token})
// The URL is identical (/share/{token}), so we probe the bundle endpoint
// first and fall back to the secret endpoint on 404. The token IS the
// credential; an optional passphrase gates the reveal on top of it.
//
// Both endpoints return:
//   200 { ... }   -> reveal
//   401 { error } -> passphrase required or incorrect
//   404 { error } -> expired / revoked / exhausted / unknown / wrong-kind
// We branch on HTTP status, never message text.

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "";

type ViewState =
  | { kind: "loading" }
  | { kind: "needsPassphrase"; incorrect: boolean }
  | { kind: "revealed"; payload: string }
  | { kind: "bundle"; payload: BundlePayload }
  | { kind: "gone" }
  | { kind: "error"; message: string };

function withPass(path: string, passphrase?: string): string {
  const qs = passphrase ? `?passphrase=${encodeURIComponent(passphrase)}` : "";
  return `${API_BASE}${path}${qs}`;
}

async function redeem(token: string, passphrase?: string): Promise<ViewState> {
  const enc = encodeURIComponent(token);

  // 1) Try the bundle endpoint first.
  let bundleRes: Response;
  try {
    bundleRes = await fetch(withPass(`/api/share-bundle/${enc}`, passphrase));
  } catch {
    return { kind: "error", message: "share.connectionError" };
  }
  if (bundleRes.ok) {
    const data = (await bundleRes.json().catch(() => null)) as BundlePayload | null;
    if (data) return { kind: "bundle", payload: data };
  } else if (bundleRes.status === 401) {
    return { kind: "needsPassphrase", incorrect: Boolean(passphrase) };
  } else if (bundleRes.status !== 404) {
    const body = await bundleRes.json().catch(() => ({}));
    return { kind: "error", message: body.error || `share.requestFailed:${bundleRes.status}` };
  }

  // 2) Not a bundle (404) — fall back to a single-secret link.
  let res: Response;
  try {
    res = await fetch(withPass(`/api/share/${enc}`, passphrase));
  } catch {
    return { kind: "error", message: "share.connectionError" };
  }
  if (res.ok) {
    const data = await res.json().catch(() => ({}));
    return { kind: "revealed", payload: typeof data.payload === "string" ? data.payload : "" };
  }
  if (res.status === 401) return { kind: "needsPassphrase", incorrect: Boolean(passphrase) };
  if (res.status === 404) return { kind: "gone" };
  const body = await res.json().catch(() => ({}));
  return { kind: "error", message: body.error || `share.requestFailed:${res.status}` };
}

// Some payloads are JSON (env-var bundles, app logins). Pretty-print those so
// they're readable; otherwise show the raw string.
function prettyPayload(payload: string): string {
  const trimmed = payload.trim();
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try {
      return JSON.stringify(JSON.parse(trimmed), null, 2);
    } catch {
      /* not JSON — fall through */
    }
  }
  return payload;
}

interface ParsedPayloadField {
  label: string;
  value: string;
  isCode?: boolean;
  isSensitive?: boolean;
}

interface ParsedPayload {
  detectedType: "password" | "cred" | "sshkey" | "app_login" | "raw";
  fields: ParsedPayloadField[];
}

function parseSecretPayload(payload: string, type?: string): ParsedPayload {
  const trimmed = payload.trim();

  // If not JSON, it's raw
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) {
    return {
      detectedType: "raw",
      fields: [{ label: "share.fields.secretValue", value: payload }]
    };
  }

  try {
    const data = JSON.parse(trimmed);
    if (typeof data !== "object" || data === null) {
      return { detectedType: "raw", fields: [{ label: "share.fields.secretValue", value: payload }] };
    }

    // Determine the type
    let resolvedType = type || "";
    if (!resolvedType) {
      if ("value" in data && Object.keys(data).length === 1) {
        resolvedType = "password";
      } else if ("username" in data && "password" in data && "private_key_pem" in data) {
        resolvedType = "sshkey";
      } else if ("username" in data && "password" in data && "app_name" in data) {
        resolvedType = "app_login";
      } else if ("username" in data && "password" in data) {
        resolvedType = "cred";
      } else if ("private_key_pem" in data) {
        resolvedType = "sshkey";
      } else {
        resolvedType = "raw";
      }
    }

    const fields: ParsedPayloadField[] = [];

    if (resolvedType === "password") {
      fields.push({ label: "share.fields.password", value: data.value || "", isSensitive: true });
    } else if (resolvedType === "cred") {
      fields.push({ label: "share.fields.username", value: data.username || "" });
      fields.push({ label: "share.fields.password", value: data.password || "", isSensitive: true });
    } else if (resolvedType === "sshkey") {
      if (data.username) fields.push({ label: "share.fields.username", value: data.username });
      if (data.private_key_pem) fields.push({ label: "share.fields.privateKey", value: data.private_key_pem, isCode: true });
      if (data.public_key) fields.push({ label: "share.fields.publicKey", value: data.public_key, isCode: true });
    } else if (resolvedType === "app_login") {
      if (data.app_name) fields.push({ label: "share.fields.appName", value: data.app_name });
      if (data.url) fields.push({ label: "share.fields.url", value: data.url });
      if (data.username) fields.push({ label: "share.fields.username", value: data.username });
      if (data.password) fields.push({ label: "share.fields.password", value: data.password || "", isSensitive: true });
      if (data.notes) fields.push({ label: "share.fields.notes", value: data.notes });
    } else {
      // Generic JSON: list key-values nicely
      for (const [k, v] of Object.entries(data)) {
        const valStr = typeof v === "object" ? JSON.stringify(v, null, 2) : String(v);
        const isMultiline = valStr.includes("\n") || valStr.length > 80;
        fields.push({
          label: k.split("_").map((w) => w.charAt(0).toUpperCase() + w.slice(1)).join(" "),
          value: valStr,
          isCode: isMultiline,
          isSensitive: k.toLowerCase().includes("password") || k.toLowerCase().includes("key") || k.toLowerCase().includes("token") || k.toLowerCase().includes("secret")
        });
      }
    }

    return {
      detectedType: resolvedType as any,
      fields
    };
  } catch {
    return {
      detectedType: "raw",
      fields: [{ label: "share.fields.secretValue", value: payload }]
    };
  }
}

function SecretPayloadViewer({
  payload,
  type,
  onCopy,
  copiedKeyPrefix,
  copiedKey,
}: {
  payload: string;
  type?: string;
  onCopy: (val: string, key: string) => void;
  copiedKeyPrefix: string;
  copiedKey: string | null;
}) {
  const { t } = useLocale();
  const { fields, detectedType } = parseSecretPayload(payload, type);
  const [revealedSensitives, setRevealedSensitives] = useState<Record<number, boolean>>({});

  const toggleReveal = (idx: number) => {
    setRevealedSensitives((prev) => ({ ...prev, [idx]: !prev[idx] }));
  };

  // If raw single-line secret
  if (detectedType === "raw" && fields.length === 1 && !fields[0].value.includes("\n")) {
    const val = fields[0].value;
    const currentKey = `${copiedKeyPrefix}-raw`;
    const isRevealed = revealedSensitives[0] ?? false;
    return (
      <div className="flex items-center gap-2 bg-[var(--bg-overlay)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-2.5">
        <span className="font-mono text-xs text-[var(--text-primary)] select-all truncate flex-1 pl-1">
          {isRevealed ? val : "••••••••••••••••"}
        </span>
        <button
          type="button"
          onClick={() => toggleReveal(0)}
          className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] px-2 py-1 cursor-pointer font-medium"
        >
          {isRevealed ? t("common.hide") : t("common.show")}
        </button>
        <Button
          variant="secondary"
          size="sm"
          onClick={() => onCopy(val, currentKey)}
          className="shrink-0"
        >
          {copiedKey === currentKey ? t("common.copied") : t("common.copy")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-3 mt-2">
      {fields.map((f, idx) => {
        const fieldKey = `${copiedKeyPrefix}-${idx}`;
        const isCopied = copiedKey === fieldKey;
        const isSensitive = f.isSensitive;
        const isRevealed = revealedSensitives[idx];

        return (
          <div key={idx} className="space-y-1.5">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-semibold text-[var(--text-secondary)] uppercase tracking-wider">
                {f.label.startsWith("share.fields.") ? t(f.label) : f.label}
              </span>
              <div className="flex items-center gap-2">
                {isSensitive && (
                  <button
                    type="button"
                    onClick={() => toggleReveal(idx)}
                    className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] cursor-pointer"
                  >
                    {isRevealed ? t("common.hide") : t("common.show")}
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => onCopy(f.value, fieldKey)}
                  className="text-xs text-[var(--accent)] hover:underline cursor-pointer font-medium"
                >
                  {isCopied ? t("common.copied") : t("common.copy")}
                </button>
              </div>
            </div>
            {f.isCode ? (
              <pre className="overflow-x-auto whitespace-pre font-mono text-xs text-[var(--text-primary)] bg-[var(--bg-overlay)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-3">
                {isSensitive && !isRevealed ? "••••••••••••••••\n••••••••••••••••\n••••••••••••••••" : f.value}
              </pre>
            ) : (
              <div className="bg-[var(--bg-overlay)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] px-3 py-2 font-mono text-xs text-[var(--text-primary)] flex items-center justify-between">
                <span className="select-all truncate">
                  {isSensitive && !isRevealed ? "••••••••••••••••" : f.value}
                </span>
              </div>
            )}
          </div>
        );
      })}

      {detectedType !== "raw" && (
        <div className="flex justify-end pt-1">
          <button
            type="button"
            onClick={() => onCopy(payload, `${copiedKeyPrefix}-raw-json`)}
            className="text-[10px] text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:underline cursor-pointer"
          >
            {copiedKey === `${copiedKeyPrefix}-raw-json` ? t("share.copiedRawJson") : t("share.copyRawJson")}
          </button>
        </div>
      )}
    </div>
  );
}

const SECTOR_BRANDING = {
  enabled: true,
  name: "SEAD",
  subName: "Secretaria de Administração do Piauí",
  logoUrl: "", // Path to your logo (e.g. "/sead-logo.svg")
};

function DepartmentIcon() {
  return (
    <svg className="w-3.5 h-3.5 text-[var(--text-secondary)]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
    </svg>
  );
}

function LocaleToggle() {
  const { locale, setLocale } = useLocale();
  return (
    <div className="flex h-8 rounded-[var(--radius-md)] border border-[var(--border-default)] overflow-hidden">
      <button
        onClick={() => setLocale("en")}
        className={`px-3 text-[11px] font-medium transition-all duration-150 cursor-pointer ${
          locale === "en"
            ? "bg-[var(--accent-muted)] text-[var(--accent)]"
            : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
        }`}
      >
        EN
      </button>
      <button
        onClick={() => setLocale("pt-BR")}
        className={`px-3 text-[11px] font-medium border-l border-[var(--border-default)] transition-all duration-150 cursor-pointer ${
          locale === "pt-BR"
            ? "bg-[var(--accent-muted)] text-[var(--accent)]"
            : "text-[var(--text-muted)] hover:text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
        }`}
      >
        PT
      </button>
    </div>
  );
}

function ThemeToggle() {
  const { theme, toggleTheme } = useTheme();
  return (
    <button
      onClick={toggleTheme}
      aria-label="Toggle theme"
      title={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"}
      className="w-8 h-8 flex items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-default)] text-[var(--text-muted)] hover:text-[var(--accent)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer"
    >
      {theme === "dark" ? (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2m0 14v2M5.05 5.05l1.41 1.41M17.54 17.54l1.41 1.41M3 12h2m14 0h2M5.05 18.95l1.41-1.41M17.54 6.46l1.41-1.41" />
          <circle cx="12" cy="12" r="4" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      ) : (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M21 12.79A9 9 0 1111.21 3a7 7 0 009.79 9.79z" />
        </svg>
      )}
    </button>
  );
}

export default function SharedSecretPage(props: { params: Promise<{ token: string }> }) {
  // Next 16: route params arrive as a Promise, unwrapped with React's use().
  const { token } = use(props.params);

  const { theme } = useTheme();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const [state, setState] = useState<ViewState>({ kind: "loading" });
  const [passphrase, setPassphrase] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);

  const attempt = useCallback(
    async (pass?: string) => {
      const next = await redeem(token, pass);
      setState(next);
    },
    [token],
  );

  useEffect(() => {
    void attempt();
  }, [attempt]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!passphrase) return;
    setSubmitting(true);
    await attempt(passphrase);
    setSubmitting(false);
  }

  async function handleCopy(value: string, key: string) {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(key);
      setTimeout(() => setCopied(null), 2000);
    } catch {
      /* clipboard unavailable — user can still select manually */
    }
  }

  // Bundles get a full-width layout (API docs need room); everything else
  // stays in the narrow centered card.
  if (state.kind === "bundle") {
    const b = state.payload;
    return (
      <main className="min-h-screen p-6 relative overflow-hidden bg-grid" style={{ background: "var(--bg-base)" }}>
        {/* Background decoration */}
        <div className="fixed inset-0 overflow-hidden pointer-events-none z-0">
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full opacity-30"
            style={{ background: `radial-gradient(circle, ${appColor || 'var(--accent)'}14 0%, transparent 70%)` }}
          />
          <div
            className="absolute -right-36 -bottom-36 select-none pointer-events-none transition-opacity duration-300"
            style={{
              width: "700px",
              height: "1000px",
              opacity: theme === "light" ? 0.15 : 0.10
            }}
          >
            <img src="/LOGO%20PIAU%C3%8D.svg" alt="" className="w-full h-full object-contain" />
          </div>
        </div>

        <div className="max-w-5xl mx-auto space-y-5 relative z-10">
          {/* Top branding bar */}
          <div className="flex items-center justify-between pb-4 border-b border-[var(--border-subtle)] animate-fade-in">
            <div className="flex items-center flex-wrap gap-2">
              {/* Product branding */}
              <div className="flex items-center gap-2">
                <div className="w-7 h-7 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden shrink-0"
                  style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
                  {appLogo ? (
                    <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
                  ) : (
                    <svg className="w-3.5 h-3.5" style={{ color: appColor }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
                    </svg>
                  )}
                </div>
                <span className="text-sm font-bold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
                  {appName}
                </span>
              </div>

              {/* Co-Branding Divider & Sector Info */}
              {SECTOR_BRANDING.enabled && (
                <>
                  <div className="w-px h-4 bg-[var(--border-default)] mx-1" />
                  <div className="flex items-center gap-1.5 pl-0.5">
                    <div className="w-6 h-6 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border-subtle)] shrink-0">
                      {SECTOR_BRANDING.logoUrl ? (
                        <img src={SECTOR_BRANDING.logoUrl} alt="" className="w-full h-full object-contain p-0.5" />
                      ) : (
                        <DepartmentIcon />
                      )}
                    </div>
                    <div className="flex flex-col min-w-0">
                      <span className="text-xs font-semibold text-[var(--text-primary)] truncate leading-tight">
                        {SECTOR_BRANDING.name}
                      </span>
                      {SECTOR_BRANDING.subName && (
                        <span className="text-[9px] text-[var(--text-muted)] truncate leading-none mt-0.5">
                          {SECTOR_BRANDING.subName}
                        </span>
                      )}
                    </div>
                  </div>
                </>
              )}
            </div>
            <div className="flex items-center gap-2">
              <LocaleToggle />
              <ThemeToggle />
            </div>
          </div>

          <div>
            <h1 className="text-lg font-semibold text-[var(--text-primary)]">{b.title || t("share.bundleTitle")}</h1>
            <p className="text-xs text-[var(--text-muted)]">{t("share.bundleDesc")}</p>
          </div>

          {b.secrets.length > 0 && (
            <div className="space-y-3">
              <h2 className="text-sm font-semibold text-[var(--text-secondary)]">{t("share.secrets")}</h2>
              {b.secrets.map((s, i) => (
                <Card key={i}>
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-sm font-medium text-[var(--text-primary)]">
                      {s.name} <span className="text-xs text-[var(--text-muted)]">({t("share.types." + s.type)})</span>
                    </span>
                  </div>
                  <SecretPayloadViewer
                    payload={s.payload}
                    type={s.type}
                    onCopy={handleCopy}
                    copiedKeyPrefix={`s-${i}`}
                    copiedKey={copied}
                  />
                </Card>
              ))}
            </div>
          )}

          {b.api_docs.map((doc, i) => (
            <div key={i} className="space-y-2">
              <div className="flex items-center gap-3">
                <h2 className="text-sm font-semibold text-[var(--text-secondary)]">{doc.name}</h2>
              </div>
              <Card className="p-0 overflow-hidden">
                <ApiReference content={doc.spec} showSidebar={false} hideTestRequest />
              </Card>
            </div>
          ))}

          {b.secrets.length === 0 && b.api_docs.length === 0 && (
            <div className={`text-sm rounded-[var(--radius-md)] p-3 ${
              theme === "light"
                ? "bg-amber-50 border border-amber-200 text-amber-700"
                : "bg-amber-500/10 border border-amber-500/25 text-amber-400"
            }`}>
              {t("share.noContent")}
            </div>
          )}

          {/* Security Notice */}
          <div className={`flex gap-2.5 items-start p-3.5 rounded-[var(--radius-md)] border text-xs ${
            theme === "light"
              ? "bg-sky-50/50 border-sky-100/80 text-sky-800"
              : "bg-sky-950/20 border-sky-900/30 text-[var(--text-muted)]"
          }`}>
            <svg className={`w-4 h-4 shrink-0 mt-0.5 ${theme === "light" ? "text-sky-600" : "text-sky-400"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <div>
              <strong className={`font-semibold block mb-0.5 ${theme === "light" ? "text-sky-900" : "text-[var(--text-secondary)]"}`}>
                {t("share.securityNotice")}
              </strong>
              {t("share.securityNoticeDesc")}
            </div>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen flex items-center justify-center p-6 relative overflow-hidden bg-grid" style={{ background: "var(--bg-base)" }}>
      {/* Background decoration */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none z-0">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full opacity-30"
          style={{ background: `radial-gradient(circle, ${appColor || 'var(--accent)'}14 0%, transparent 70%)` }}
        />
        <div
          className="absolute -right-36 -bottom-36 select-none pointer-events-none transition-opacity duration-300"
          style={{
            width: "700px",
            height: "1000px",
            opacity: theme === "light" ? 0.15 : 0.10
          }}
        >
          <img src="/LOGO%20PIAU%C3%8D.svg" alt="" className="w-full h-full object-contain" />
        </div>
      </div>

      <div className="w-full max-w-md relative z-10">
        {/* Branding & Theme Toggle */}
        <div className="flex items-center justify-between mb-6 animate-fade-in">
          <div className="flex items-center flex-wrap gap-2">
            {/* Product branding */}
            <div className="flex items-center gap-2">
              <div className="w-8 h-8 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden shrink-0"
                style={{ backgroundColor: `${appColor}15`, border: `1px solid ${appColor}40` }}>
                {appLogo ? (
                  <img src={appLogo} alt="" className="w-full h-full object-contain p-0.5" />
                ) : (
                  <svg className="w-4 h-4" style={{ color: appColor }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
                  </svg>
                )}
              </div>
              <span className="text-base font-bold text-[var(--text-primary)] tracking-wide" style={{ fontFamily: "var(--font-display)" }}>
                {appName}
              </span>
            </div>

            {/* Co-Branding Divider & Sector Info */}
            {SECTOR_BRANDING.enabled && (
              <>
                <div className="w-px h-5 bg-[var(--border-default)] mx-1" />
                <div className="flex items-center gap-1.5 pl-0.5">
                  <div className="w-6 h-6 rounded-[var(--radius-sm)] flex items-center justify-center overflow-hidden bg-[var(--bg-elevated)] border border-[var(--border-subtle)] shrink-0">
                    {SECTOR_BRANDING.logoUrl ? (
                      <img src={SECTOR_BRANDING.logoUrl} alt="" className="w-full h-full object-contain p-0.5" />
                    ) : (
                      <DepartmentIcon />
                    )}
                  </div>
                  <div className="flex flex-col min-w-0">
                    <span className="text-xs font-semibold text-[var(--text-primary)] truncate leading-tight">
                      {SECTOR_BRANDING.name}
                    </span>
                    {SECTOR_BRANDING.subName && (
                      <span className="text-[9px] text-[var(--text-muted)] truncate leading-none mt-0.5">
                        {SECTOR_BRANDING.subName}
                      </span>
                    )}
                  </div>
                </div>
              </>
            )}
          </div>
          <div className="flex items-center gap-2">
            <LocaleToggle />
            <ThemeToggle />
          </div>
        </div>

        <Card>
          <h1 className="text-base font-semibold text-[var(--text-primary)] mb-1">{t("share.secretTitle")}</h1>
          <p className="text-xs text-[var(--text-muted)] mb-5">
            {t("share.secretDesc")}
          </p>

          {state.kind === "loading" && <p className="text-sm text-[var(--text-muted)]">{t("common.loading")}</p>}

          {state.kind === "needsPassphrase" && (
            <form onSubmit={handleSubmit} className="space-y-3">
              <p className="text-sm text-[var(--text-secondary)]">
                {t("share.protected")}
              </p>
              <Input
                type="password"
                autoFocus
                value={passphrase}
                onChange={(e) => setPassphrase(e.target.value)}
                placeholder={t("share.passphrase")}
                autoComplete="off"
                error={state.incorrect ? t("share.incorrectPassphrase") : undefined}
              />
              <Button type="submit" loading={submitting} disabled={!passphrase} className="w-full">
                {t("share.unlock")}
              </Button>
            </form>
          )}

          {state.kind === "revealed" && (
            <div className="space-y-4">
              <SecretPayloadViewer
                payload={state.payload}
                onCopy={handleCopy}
                copiedKeyPrefix="secret"
                copiedKey={copied}
              />
              <div className={`flex gap-2.5 items-start p-3 rounded-[var(--radius-md)] border text-[11px] ${
                theme === "light"
                  ? "bg-sky-50/50 border-sky-100/80 text-sky-800"
                  : "bg-sky-950/10 border-sky-900/20 text-[var(--text-muted)]"
              }`}>
                <svg className={`w-3.5 h-3.5 shrink-0 mt-0.5 ${theme === "light" ? "text-sky-600" : "text-sky-400"}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                </svg>
                <div>
                  <strong className={`font-semibold block mb-0.5 ${theme === "light" ? "text-sky-900" : "text-[var(--text-secondary)]"}`}>
                    {t("share.securityNotice")}
                  </strong>
                  {t("share.securityNoticeDescSingle")}
                </div>
              </div>
            </div>
          )}

          {state.kind === "gone" && (
            <div className={`text-sm rounded-[var(--radius-md)] p-3 ${
              theme === "light"
                ? "bg-amber-50 border border-amber-200 text-amber-700"
                : "bg-amber-500/10 border border-amber-500/25 text-amber-400"
            }`}>
              {t("share.expired")}
            </div>
          )}

          {state.kind === "error" && (
            <div className={`text-sm rounded-[var(--radius-md)] p-3 ${
              theme === "light"
                ? "bg-red-50 border border-red-200 text-red-700"
                : "bg-red-500/10 border border-red-500/25 text-red-400"
            }`}>
              {state.message.startsWith("share.requestFailed:")
                ? t("share.requestFailed", { status: state.message.split(":")[1] })
                : state.message.startsWith("share.")
                ? t(state.message)
                : state.message}
            </div>
          )}
        </Card>
      </div>
    </main>
  );
}
