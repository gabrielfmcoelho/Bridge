"use client";

import { createContext, useContext, useEffect, useState, type ReactNode } from "react";
import { appearanceAPI } from "@/lib/api";

interface AppearanceSettings {
  appName: string;
  appColor: string;
  appLogo: string;
}

interface AppearanceContextValue extends AppearanceSettings {
  refresh: () => Promise<void>;
}

const defaults: AppearanceSettings = {
  appName: "Bridge",
  appColor: "#06b6d4",
  appLogo: "",
};

const AppearanceContext = createContext<AppearanceContextValue>({
  ...defaults,
  refresh: async () => {},
});

export function AppearanceProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState<AppearanceSettings>(defaults);

  const refresh = async () => {
    try {
      const data = await appearanceAPI.get();
      setSettings({
        appName: data.app_name || defaults.appName,
        appColor: data.app_color || defaults.appColor,
        appLogo: data.app_logo || "",
      });
    } catch {
      // Keep defaults if API not available yet (setup phase)
    }
  };

  useEffect(() => {
    refresh();
  }, []);

  // Apply accent color as CSS variable override whenever it changes
  useEffect(() => {
    const color = settings.appColor;
    const doc = document.documentElement;
    doc.style.setProperty("--accent", color);
    doc.style.setProperty("--accent-hover", color);
    doc.style.setProperty("--accent-muted", hexToRgba(color, 0.15));
    doc.style.setProperty("--accent-glow", hexToRgba(color, 0.08));
    doc.style.setProperty("--shadow-glow", `0 0 40px ${hexToRgba(color, 0.08)}`);
  }, [settings.appColor]);

  return (
    <AppearanceContext.Provider value={{ ...settings, refresh }}>
      {children}
    </AppearanceContext.Provider>
  );
}

export function useAppearance() {
  return useContext(AppearanceContext);
}

function hexToRgba(hex: string, alpha: number): string {
  const cleaned = hex.replace("#", "");
  const r = parseInt(cleaned.substring(0, 2), 16);
  const g = parseInt(cleaned.substring(2, 4), 16);
  const b = parseInt(cleaned.substring(4, 6), 16);
  if (isNaN(r) || isNaN(g) || isNaN(b)) return `rgba(6, 182, 212, ${alpha})`;
  return `rgba(${r}, ${g}, ${b}, ${alpha})`;
}
