"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { authAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

export default function SetupPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { refresh } = useAuth();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await authAPI.setup({ username, password, display_name: displayName });
      await refresh();
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Setup failed");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4 bg-grid" style={{ background: "var(--bg-base)" }}>
      {/* Background glow */}
      <div className="fixed inset-0 overflow-hidden pointer-events-none">
        <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] rounded-full opacity-30"
          style={{ background: `radial-gradient(circle, ${appColor}14 0%, transparent 70%)` }}
        />
        <div className="absolute top-0 left-0 right-0 h-px" style={{ background: `linear-gradient(to right, transparent, ${appColor}33, transparent)` }} />
      </div>

      <div className="w-full max-w-md relative animate-slide-up">
        {/* Branding */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-14 h-14 rounded-2xl mb-4 overflow-hidden"
            style={{ backgroundColor: `${appColor}18`, border: `1px solid ${appColor}33`, boxShadow: `0 0 30px ${appColor}1a` }}>
            {appLogo ? (
              <img src={appLogo} alt="" className="w-full h-full object-contain p-1.5" />
            ) : (
              <svg className="w-7 h-7" style={{ color: appColor }} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M4 17l6-6-6-6m8 14h8" />
              </svg>
            )}
          </div>
          <h1 className="text-3xl font-bold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>
            {appName}
          </h1>
          <p className="text-[var(--text-muted)] mt-2 text-sm">{t("auth.setupDescription")}</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="glass border border-[var(--border-default)] rounded-[var(--radius-xl)] p-6 space-y-5 shadow-[var(--shadow-lg)]">
          <h2 className="text-lg font-semibold text-[var(--text-primary)]">{t("auth.setupTitle")}</h2>

          {error && (
            <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3 animate-slide-down flex items-center gap-2">
              <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
              </svg>
              {error}
            </div>
          )}

          <Input
            label={t("auth.displayName")}
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            placeholder="Administrator"
          />
          <Input
            label={t("auth.username")}
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            placeholder="admin"
            required
          />
          <Input
            label={t("auth.password")}
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />

          <Button type="submit" loading={loading} className="w-full">
            {t("auth.createAccount")}
          </Button>
        </form>

        <p className="text-center mt-6 text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
          Initial setup - create the master administrator account
        </p>
      </div>
    </div>
  );
}
