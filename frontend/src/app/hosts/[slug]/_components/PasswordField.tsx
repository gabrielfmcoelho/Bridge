"use client";

import { useState } from "react";
import { hostsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";

export default function PasswordField({ slug }: { slug: string }) {
  const { t } = useLocale();
  const [visible, setVisible] = useState(false);
  const [password, setPassword] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const hasPassword = typeof password === "string" && password.length > 0;

  const reveal = async () => {
    if (loading) return;
    setError(null);
    if (hasPassword) {
      setVisible(!visible);
      return;
    }
    setLoading(true);
    try {
      const res = await hostsAPI.getPassword(slug);
      if (typeof res?.password !== "string" || res.password.length === 0) {
        setPassword(null);
        setVisible(false);
        setError(t("host.passwordEmpty"));
        return;
      }
      setPassword(res.password);
      setVisible(true);
    } catch (err) {
      setPassword(null);
      setVisible(false);
      setError(err instanceof Error && err.message ? err.message : t("host.passwordRevealFailed"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div>
      <span className="text-[var(--text-muted)] text-xs font-medium">{t("auth.password")}</span>
      <div className="flex items-center gap-1.5 mt-0.5">
        <p className="text-[var(--text-primary)] truncate" style={{ fontFamily: "var(--font-mono)" }}>
          {visible && hasPassword ? password : "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"}
        </p>
        <button
          type="button"
          onClick={reveal}
          disabled={loading}
          className="text-[var(--text-faint)] hover:text-[var(--accent)] transition-colors shrink-0 disabled:opacity-40 disabled:cursor-not-allowed"
          title={visible ? t("common.hide") : t("common.show")}
          aria-label={visible ? t("common.hide") : t("common.show")}
        >
          {loading ? (
            <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="3" strokeOpacity="0.25" />
              <path d="M12 2a10 10 0 019.17 6" stroke="currentColor" strokeWidth="3" strokeLinecap="round" />
            </svg>
          ) : (
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              {visible ? (
                <path strokeLinecap="round" strokeLinejoin="round" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
              )}
            </svg>
          )}
        </button>
        {hasPassword && (
          <button
            type="button"
            onClick={() => { setPassword(null); setVisible(false); setError(null); }}
            className="text-[var(--text-faint)] hover:text-[var(--accent)] transition-colors shrink-0"
            title={t("common.refresh")}
            aria-label={t("common.refresh")}
          >
            <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        )}
      </div>
      {error && (
        <p className="mt-1 text-[10px] text-red-400" style={{ fontFamily: "var(--font-mono)" }}>{error}</p>
      )}
    </div>
  );
}
