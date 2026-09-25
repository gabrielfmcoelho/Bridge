"use client";

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { useLocale } from "./LocaleContext";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import { ICON_PATHS } from "@/lib/icon-paths";

export type FlagAppearance = "success" | "info" | "warning" | "error";

export interface FlagInput {
  appearance: FlagAppearance;
  title: string;
  description?: string;
  /** At most two (ADS). */
  actions?: { label: string; onClick: () => void }[];
}

interface Flag extends FlagInput {
  id: number;
}

type ShowFlag = (flag: FlagInput) => void;

const FlagContext = createContext<ShowFlag | null>(null);

/**
 * Transient result of something the admin started (a sync finished, a save
 * failed): `const flag = useFlag(); flag({ appearance: "success", title })`.
 * Page-level problems stay StatusAlert; this is for outcomes that shouldn't
 * push the page down.
 */
export function useFlag(): ShowFlag {
  const ctx = useContext(FlagContext);
  if (!ctx) throw new Error("useFlag needs <FlagProvider>");
  return ctx;
}

// ADS: never auto-dismiss what the admin must act on; 8s minimum otherwise.
const AUTO_DISMISS_MS = 8000;
const autoDismiss = (a: FlagAppearance) => a === "success" || a === "info";

const tone: Record<FlagAppearance, { icon: string; color: string }> = {
  success: { icon: ICON_PATHS.checkCircle, color: "var(--success)" },
  info: { icon: ICON_PATHS.infoCircle, color: "var(--info)" },
  warning: { icon: ICON_PATHS.alert, color: "var(--warning)" },
  error: { icon: ICON_PATHS.xCircle, color: "var(--danger)" },
};

export function FlagProvider({ children }: { children: ReactNode }) {
  const [flags, setFlags] = useState<Flag[]>([]);
  const dismiss = useCallback((id: number) => setFlags((fs) => fs.filter((f) => f.id !== id)), []);
  const show = useCallback<ShowFlag>((f) => {
    // ponytail: newest 5 kept; older ones drop off rather than queue.
    setFlags((fs) => [{ ...f, actions: f.actions?.slice(0, 2), id: Date.now() + Math.random() }, ...fs].slice(0, 5));
  }, []);

  return (
    <FlagContext.Provider value={show}>
      {children}
      {/* Bottom-left, newest on top; clear of the collapsed rail and the phone nav. */}
      <div className="fixed z-[90] left-4 md:left-20 bottom-20 md:bottom-4 flex flex-col gap-2 w-[min(24rem,calc(100vw-2rem))] pointer-events-none">
        {flags.map((f) => (
          <FlagItem key={f.id} flag={f} onDismiss={() => dismiss(f.id)} />
        ))}
      </div>
    </FlagContext.Provider>
  );
}

function FlagItem({ flag, onDismiss }: { flag: Flag; onDismiss: () => void }) {
  const { t } = useLocale();
  useEffect(() => {
    if (!autoDismiss(flag.appearance)) return;
    const id = setTimeout(onDismiss, AUTO_DISMISS_MS);
    return () => clearTimeout(id);
  }, [flag.appearance, onDismiss]);

  const { icon, color } = tone[flag.appearance];
  const urgent = flag.appearance === "error" || flag.appearance === "warning";
  return (
    <div
      role={urgent ? "alert" : "status"}
      className="pointer-events-auto flex items-start gap-3 rounded-[var(--radius-lg)] border border-[var(--border-default)] bg-[var(--bg-surface)] p-3 shadow-[var(--shadow-lg)] animate-slide-up"
    >
      <Icon path={icon} className="w-5 h-5 shrink-0 mt-0.5" style={{ color }} />
      <div className="min-w-0 flex-1">
        <p className="text-sm font-semibold text-[var(--text-primary)]">{flag.title}</p>
        {flag.description && <p className="text-sm text-[var(--text-secondary)] mt-0.5">{flag.description}</p>}
        {flag.actions && flag.actions.length > 0 && (
          <div className="flex gap-3 mt-2">
            {flag.actions.map((a) => (
              <button
                key={a.label}
                type="button"
                onClick={() => { a.onClick(); onDismiss(); }}
                className="text-sm font-medium text-[var(--accent)] hover:underline"
              >
                {a.label}
              </button>
            ))}
          </div>
        )}
      </div>
      <IconButton label={t("common.close")} onClick={onDismiss} className="-mr-1 -mt-1 shrink-0">
        <Icon path={ICON_PATHS.close} className="w-4 h-4" />
      </IconButton>
    </div>
  );
}
