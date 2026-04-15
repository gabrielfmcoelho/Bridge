"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";

/** SVG icons for known auth providers. */
function ProviderIcon({ name, color, size = 20 }: { name: string; color: string; size?: number }) {
  if (name === "gitlab") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
        <path d="M22.65 14.39L12 22.13 1.35 14.39a.84.84 0 01-.3-.94l1.22-3.78 2.44-7.51a.42.42 0 01.82 0l2.44 7.51h8.06l2.44-7.51a.42.42 0 01.82 0l2.44 7.51 1.22 3.78a.84.84 0 01-.3.94z" fill={color} />
      </svg>
    );
  }
  if (name === "keycloak") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
        <path d="M3 3h18v18H3V3z" rx="3" fill={color} fillOpacity={0.12} />
        <path d="M17.5 6.5h-3.25l-2.5 3.75L14.25 14H17.5l-2.5-3.75L17.5 6.5z" fill={color} />
        <path d="M6.5 6.5h3.25l2.5 3.75L9.75 14H6.5l2.5-3.75L6.5 6.5z" fill={color} />
        <path d="M9.75 14l2.25 3.5 2.25-3.5H9.75z" fill={color} />
      </svg>
    );
  }
  if (name === "ldap") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        <path d="M9 12l2 2 4-4" />
      </svg>
    );
  }
  // Fallback: colored circle
  return (
    <span className="inline-block rounded-full" style={{ width: size, height: size, backgroundColor: color }} />
  );
}

export default function LoginPage() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [activeProvider, setActiveProvider] = useState("local");
  const [showCredentials, setShowCredentials] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [rememberMe, setRememberMe] = useState(false);

  // Restore saved username on mount.
  useEffect(() => {
    const saved = localStorage.getItem("sshcm_remember_user");
    if (saved) {
      setUsername(saved);
      setRememberMe(true);
    }
  }, []);
  const { login, providers, refresh } = useAuth();
  const router = useRouter();
  const { t } = useLocale();
  const { appName, appColor, appLogo } = useAppearance();
  const searchParams = typeof window !== "undefined" ? new URLSearchParams(window.location.search) : null;

  // Handle OAuth callback results.
  useState(() => {
    if (searchParams?.get("auth") === "success") {
      refresh().then(() => router.push("/"));
    } else if (searchParams?.get("auth") === "error") {
      setError(searchParams.get("message") || "Authentication failed");
    }
  });

  const directProviders = providers.filter((p) => p.type === "direct");
  const oauthProviders = providers.filter((p) => p.type === "oauth");
  const hasSSO = oauthProviders.length > 0;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      if (rememberMe) {
        localStorage.setItem("sshcm_remember_user", username);
      } else {
        localStorage.removeItem("sshcm_remember_user");
      }
      await login(username, password, activeProvider);
      router.push("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
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
          <p className="text-[var(--text-muted)] mt-2 text-sm">
            {hasSSO ? t("auth.loginDescriptionSSO") : t("auth.loginDescription")}
          </p>
        </div>

        <div className="glass border border-[var(--border-default)] rounded-[var(--radius-xl)] p-6 space-y-5 shadow-[var(--shadow-lg)]">
          {error && (
            <div className="bg-red-500/10 border border-red-500/25 text-red-400 text-sm rounded-[var(--radius-md)] p-3 animate-slide-down flex items-center gap-2">
              <svg className="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
              </svg>
              {error}
            </div>
          )}

          {/* SSO-first layout: OAuth buttons are primary */}
          {hasSSO && (
            <>
              <div className="space-y-2.5">
                {oauthProviders.map((p) => (
                  <a
                    key={p.name}
                    href={`/api/auth/oauth/${p.name}/authorize`}
                    className="flex items-center justify-center gap-2.5 w-full py-3 px-4 rounded-[var(--radius-md)] text-sm font-semibold transition-all border-2 hover:shadow-md"
                    style={{
                      borderColor: p.color,
                      color: "var(--text-primary)",
                      backgroundColor: `${p.color}12`,
                    }}
                    onMouseEnter={(e) => {
                      e.currentTarget.style.backgroundColor = `${p.color}22`;
                    }}
                    onMouseLeave={(e) => {
                      e.currentTarget.style.backgroundColor = `${p.color}12`;
                    }}
                  >
                    <ProviderIcon name={p.name} color={p.color} />
                    {t("auth.signInWith", { provider: p.label })}
                  </a>
                ))}
              </div>

              {/* Collapsible credentials section */}
              <div>
                <button
                  type="button"
                  onClick={() => setShowCredentials(!showCredentials)}
                  className="flex items-center justify-center gap-2 w-full text-[11px] text-[var(--text-faint)] uppercase tracking-wider hover:text-[var(--text-muted)] transition-colors py-1"
                >
                  <div className="flex-1 h-px bg-[var(--border-subtle)]" />
                  <span className="flex items-center gap-1.5 px-2">
                    {t("auth.orUseCredentials")}
                    <svg
                      className={`w-3 h-3 transition-transform ${showCredentials ? "rotate-180" : ""}`}
                      fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                    </svg>
                  </span>
                  <div className="flex-1 h-px bg-[var(--border-subtle)]" />
                </button>
              </div>
            </>
          )}

          {/* Credentials form — always visible when no SSO, collapsible when SSO is active */}
          {(!hasSSO || showCredentials) && (
            <form onSubmit={handleSubmit} className="space-y-4">
              {!hasSSO && (
                <h2 className="text-lg font-semibold text-[var(--text-primary)]">{t("auth.loginTitle")}</h2>
              )}

              {/* Provider selector tabs (only show if more than one direct provider) */}
              {directProviders.length > 1 && (
                <div className="flex gap-1 p-1 bg-[var(--bg-elevated)] rounded-[var(--radius-md)]">
                  {directProviders.map((p) => (
                    <button
                      key={p.name}
                      type="button"
                      onClick={() => setActiveProvider(p.name)}
                      className={`flex-1 py-1.5 px-3 text-xs font-medium rounded-[var(--radius-sm)] transition-all flex items-center justify-center gap-1.5 ${
                        activeProvider === p.name
                          ? "bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-sm"
                          : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
                      }`}
                    >
                      <ProviderIcon name={p.name} color={activeProvider === p.name ? p.color : "var(--text-muted)"} size={14} />
                      {p.label}
                    </button>
                  ))}
                </div>
              )}

              <Input
                label={t("auth.username")}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus={!hasSSO}
                placeholder="admin"
              />
              <div className="relative">
                <Input
                  label={t("auth.password")}
                  type={showPassword ? "text" : "password"}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                  className="pr-9"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-2.5 bottom-[9px] text-[var(--text-faint)] hover:text-[var(--text-secondary)] transition-colors"
                  tabIndex={-1}
                >
                  {showPassword ? (
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3.98 8.223A10.477 10.477 0 001.934 12c1.292 4.338 5.31 7.5 10.066 7.5.993 0 1.953-.138 2.863-.395M6.228 6.228A10.45 10.45 0 0112 4.5c4.756 0 8.773 3.162 10.065 7.498a10.523 10.523 0 01-4.293 5.774M6.228 6.228L3 3m3.228 3.228l3.65 3.65m7.894 7.894L21 21m-3.228-3.228l-3.65-3.65m0 0a3 3 0 10-4.243-4.243m4.242 4.242L9.88 9.88" />
                    </svg>
                  ) : (
                    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 010-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178z" />
                      <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                    </svg>
                  )}
                </button>
              </div>

              <label className="flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={rememberMe}
                  onChange={(e) => setRememberMe(e.target.checked)}
                  className="w-3.5 h-3.5 rounded border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--accent)] focus:ring-[var(--accent)] focus:ring-offset-0"
                />
                <span className="text-xs text-[var(--text-muted)]">{t("auth.rememberMe")}</span>
              </label>

              <Button type="submit" loading={loading} className="w-full">
                {t("auth.signIn")}
              </Button>
            </form>
          )}
        </div>

        {/* Footer */}
        <p className="text-center mt-6 text-[10px] text-[var(--text-faint)]" style={{ fontFamily: "var(--font-mono)" }}>
          IT Asset Management
        </p>
      </div>
    </div>
  );
}
