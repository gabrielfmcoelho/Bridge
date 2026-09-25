"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useState, useRef } from "react";
import { appearanceAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAppearance } from "@/contexts/AppearanceContext";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import Input from "@/components/ui/Input";

const PRESET_COLORS = [
  { label: "Cyan", value: "#06b6d4" },
  { label: "Blue", value: "#3b82f6" },
  { label: "Violet", value: "#8b5cf6" },
  { label: "Purple", value: "#a855f7" },
  { label: "Pink", value: "#ec4899" },
  { label: "Rose", value: "#f43f5e" },
  { label: "Orange", value: "#f97316" },
  { label: "Amber", value: "#f59e0b" },
  { label: "Emerald", value: "#10b981" },
  { label: "Teal", value: "#14b8a6" },
];

export default function AppearanceTab() {
  const { t } = useLocale();
  const appearance = useAppearance();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [appName, setAppName] = useState(appearance.appName);
  const [appColor, setAppColor] = useState(appearance.appColor);
  const [appLogo, setAppLogo] = useState(appearance.appLogo);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    setSuccess(false);
    try {
      await appearanceAPI.update({ app_name: appName, app_color: appColor, app_logo: appLogo });
      await appearance.refresh();
      setSuccess(true);
      setTimeout(() => setSuccess(false), 2000);
    } catch {
      // error handled silently
    } finally {
      setSaving(false);
    }
  };

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const result = await appearanceAPI.uploadLogo(file);
      setAppLogo(result.logo);
      await appearance.refresh();
    } catch {
      // error handled silently
    } finally {
      setUploading(false);
      if (fileInputRef.current) fileInputRef.current.value = "";
    }
  };

  const handleRemoveLogo = async () => {
    try {
      await appearanceAPI.deleteLogo();
      setAppLogo("");
      await appearance.refresh();
    } catch {
      // error handled silently
    }
  };

  return (
    <div className="space-y-6 max-w-2xl">
      {/* App Name */}
      <div className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
      <SectionCard as="h3" title={t("settings.appName")} description={t("settings.appNameDescription")}>
        <Input
          value={appName}
          onChange={(e) => setAppName(e.target.value)}
          placeholder="Bridge"
          className="max-w-xs"
        />
      </SectionCard>
      </div>

      {/* Main Color */}
      <div className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
      <SectionCard as="h3" title={t("settings.mainColor")} description={t("settings.mainColorDescription")}>

        {/* Preset swatches */}
        <div className="flex flex-wrap gap-2 mb-4">
          {PRESET_COLORS.map((c) => (
            <button
              key={c.value}
              onClick={() => setAppColor(c.value)}
              className="group relative w-8 h-8 rounded-[var(--radius-sm)] border-2 transition duration-150 hover:scale-110"
              style={{
                backgroundColor: c.value,
                borderColor: appColor === c.value ? "var(--text-primary)" : "transparent",
                boxShadow: appColor === c.value ? `0 0 12px ${c.value}40` : "none",
              }}
              title={c.label}
            >
              {appColor === c.value && (
                <Icon path={ICON_PATHS.check} className="w-4 h-4 text-white absolute inset-0 m-auto drop-shadow" strokeWidth={3} />
              )}
            </button>
          ))}
        </div>

        {/* Custom color picker */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <input
              type="color"
              value={appColor}
              onChange={(e) => setAppColor(e.target.value)}
              className="w-10 h-10 rounded-[var(--radius-sm)] border border-[var(--border-default)] cursor-pointer bg-transparent"
            />
          </div>
          <Input
            value={appColor}
            onChange={(e) => setAppColor(e.target.value)}
            placeholder="#06b6d4"
            className="max-w-[140px] font-mono"
          />
          <div className="h-8 flex-1 rounded-[var(--radius-sm)]" style={{ background: `linear-gradient(135deg, ${appColor}, ${appColor}80)` }} />
        </div>
      </SectionCard>
      </div>

      {/* Logo */}
      <div className="stagger-in" style={{ "--i": 2 } as React.CSSProperties}>
      <SectionCard as="h3" title={t("settings.logo")} description={t("settings.logoDescription")}>

        <div className="flex items-center gap-4">
          {/* Preview */}
          <div
            className="w-16 h-16 rounded-[var(--radius-lg)] border border-[var(--border-default)] flex items-center justify-center overflow-hidden"
            style={{ backgroundColor: "var(--bg-elevated)" }}
          >
            {appLogo ? (
              <img src={appLogo} alt={t("settings.logoAlt")} className="w-full h-full object-contain p-1" />
            ) : (
              <Icon path={ICON_PATHS.prompt} className="w-7 h-7" strokeWidth={1.5} style={{ color: "var(--text-faint)" }} />
            )}
          </div>

          <div className="flex flex-col gap-2">
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              onChange={handleLogoUpload}
              className="hidden"
            />
            <Button
              size="sm"
              variant="secondary"
              loading={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              {t("settings.uploadLogo")}
            </Button>
            {appLogo && (
              <button
                onClick={handleRemoveLogo}
                className="text-xs hover:text-[var(--danger)] transition-colors text-left"
                style={{ color: "var(--text-faint)" }}
              >
                {t("settings.removeLogo")}
              </button>
            )}
          </div>
        </div>
      </SectionCard>
      </div>

      {/* Save button */}
      <div className="flex items-center gap-3 stagger-in" style={{ "--i": 3 } as React.CSSProperties}>
        <Button onClick={handleSave} loading={saving}>
          {t("common.save")}
        </Button>
        {success && (
          <span className="text-xs animate-fade-in" style={{ color: "var(--success)" }}>
            {t("settings.saved")}
          </span>
        )}
      </div>
    </div>
  );
}
