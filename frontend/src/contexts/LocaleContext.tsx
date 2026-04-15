"use client";

import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from "react";
import en from "@/messages/en.json";
import ptBR from "@/messages/pt-BR.json";

type Messages = Record<string, string>;

function flattenMessages(obj: Record<string, unknown>, prefix = ""): Messages {
  const result: Messages = {};
  for (const key in obj) {
    const fullKey = prefix ? `${prefix}.${key}` : key;
    if (typeof obj[key] === "object" && obj[key] !== null) {
      Object.assign(result, flattenMessages(obj[key] as Record<string, unknown>, fullKey));
    } else {
      result[fullKey] = String(obj[key]);
    }
  }
  return result;
}

const allMessages: Record<string, Messages> = {
  en: flattenMessages(en),
  "pt-BR": flattenMessages(ptBR),
};

const localeBcp47: Record<string, string> = {
  en: "en-US",
  "pt-BR": "pt-BR",
};

interface LocaleState {
  locale: string;
  setLocale: (locale: string) => void;
  t: (key: string, vars?: Record<string, string>) => string;
  formatDate: (date: string | Date, style?: "short" | "long") => string;
  formatDateTime: (date: string | Date) => string;
}

const LocaleContext = createContext<LocaleState | null>(null);

export function LocaleProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState("en");

  // Hydrate from localStorage after mount to avoid SSR/client mismatch.
  useEffect(() => {
    const stored = localStorage.getItem("sshcm_locale");
    if (stored && stored !== "en") {
      setLocaleState(stored);
    }
  }, []);

  const setLocale = useCallback((l: string) => {
    setLocaleState(l);
    if (typeof window !== "undefined") {
      localStorage.setItem("sshcm_locale", l);
    }
  }, []);

  const t = useCallback(
    (key: string, vars?: Record<string, string>) => {
      let msg = allMessages[locale]?.[key] || allMessages["en"]?.[key] || key;
      if (vars) {
        for (const [k, v] of Object.entries(vars)) {
          msg = msg.replace(`{${k}}`, v);
        }
      }
      return msg;
    },
    [locale]
  );

  const formatDate = useCallback(
    (date: string | Date, style: "short" | "long" = "short") => {
      const d = typeof date === "string" ? new Date(date) : date;
      if (isNaN(d.getTime())) return "-";
      const bcp47 = localeBcp47[locale] || locale;
      if (style === "long") {
        return d.toLocaleDateString(bcp47, { day: "2-digit", month: "short", year: "numeric" });
      }
      return d.toLocaleDateString(bcp47, { day: "2-digit", month: "2-digit", year: "numeric" });
    },
    [locale]
  );

  const formatDateTime = useCallback(
    (date: string | Date) => {
      const d = typeof date === "string" ? new Date(date) : date;
      if (isNaN(d.getTime())) return "-";
      const bcp47 = localeBcp47[locale] || locale;
      return d.toLocaleString(bcp47, {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    },
    [locale]
  );

  return (
    <LocaleContext.Provider value={{ locale, setLocale, t, formatDate, formatDateTime }}>
      {children}
    </LocaleContext.Provider>
  );
}

export function useLocale() {
  const ctx = useContext(LocaleContext);
  if (!ctx) throw new Error("useLocale must be used within LocaleProvider");
  return ctx;
}
