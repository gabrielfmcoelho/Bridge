"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useState, useRef } from "react";
import { importAPI } from "@/lib/api";
import type { ImportResult } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import PillButton from "@/components/ui/PillButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import FormError from "@/components/ui/FormError";

export default function ImportTab() {
  const { t } = useLocale();
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [importType, setImportType] = useState<"hosts" | "dns">("hosts");
  const [fileData, setFileData] = useState<Record<string, unknown>[] | null>(null);
  const [fileName, setFileName] = useState("");
  const [parseError, setParseError] = useState("");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [importing, setImporting] = useState(false);

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setResult(null);
    setParseError("");
    setFileName(file.name);

    const reader = new FileReader();
    reader.onload = (ev) => {
      try {
        const raw = JSON.parse(ev.target?.result as string);
        const arr = Array.isArray(raw) ? raw : raw.data ? raw.data : null;
        if (!arr || !Array.isArray(arr) || arr.length === 0) {
          setParseError("JSON must be an array of objects, or an object with a \"data\" array field");
          setFileData(null);
          return;
        }
        // Strip internal metadata fields
        const cleaned = arr.map((item: Record<string, unknown>) => {
          const copy = { ...item };
          for (const key of Object.keys(copy)) {
            if (key.startsWith("_")) delete copy[key];
          }
          return copy;
        });
        setFileData(cleaned);
      } catch {
        setParseError("Invalid JSON file");
        setFileData(null);
      }
    };
    reader.readAsText(file);
  };

  const handleImport = async () => {
    if (!fileData) return;
    setImporting(true);
    setResult(null);
    try {
      const res = importType === "hosts"
        ? await importAPI.hosts(fileData)
        : await importAPI.dns(fileData);
      setResult(res);
      queryClient.invalidateQueries({ queryKey: [importType === "hosts" ? "hosts" : "dns"] });
    } catch (err) {
      setParseError(err instanceof Error ? err.message : "Import failed");
    } finally {
      setImporting(false);
    }
  };

  const reset = () => {
    setFileData(null);
    setFileName("");
    setParseError("");
    setResult(null);
    if (fileInputRef.current) fileInputRef.current.value = "";
  };

  // Detect first field to auto-suggest type
  const detectedType = fileData && fileData.length > 0
    ? ("domain" in fileData[0] ? "dns" : "hosts")
    : null;

  return (
    <div className="space-y-6 max-w-2xl">
      {/* Import type selector */}
      <div className="stagger-in" style={{ "--i": 0 } as React.CSSProperties}>
      <SectionCard as="h3" title={t("settings.importType")}>
        <div className="flex gap-2">
          <PillButton size="lg" active={importType === "hosts"} onClick={() => setImportType("hosts")}>
            <Icon path={ICON_PATHS.serverStack} />
            {t("nav.hosts")}
          </PillButton>
          <PillButton size="lg" active={importType === "dns"} onClick={() => setImportType("dns")}>
            <Icon path={ICON_PATHS.globeMeridian} />
            {t("nav.dns")}
          </PillButton>
        </div>
      </SectionCard>
      </div>

      {/* File upload */}
      <div className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
      <SectionCard
        as="h3"
        title={t("settings.importer.jsonFile")}
        description={importType === "hosts" ? t("settings.importer.hostsHint") : t("settings.importer.dnsHint")}
      >

        {/* JSON example */}
        <details className="mb-4 group">
          <summary className="text-xs text-[var(--accent)] cursor-pointer hover:underline font-medium">
            {t("settings.importer.showExample")}
          </summary>
          <pre className="mt-2 p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-xs text-[var(--text-secondary)] overflow-x-auto font-mono">
{importType === "hosts" ? `[
  {
    "nickname": "My Server",
    "oficial_slug": "MY-SERVER",
    "hostname": "10.0.1.10",
    "hospedagem": "ETIPI",
    "user": "admin",
    "password": "secret123",
    "has_key": false,
    "situacao": "active",
    "setor_responsavel": "SEAD/NTGD",
    "responsavel_interno": "John Doe",
    "description": "Production server",
    "tags": ["prod", "web"]
  }
]` : `[
  {
    "domain": "app.example.gov.br",
    "has_https": true,
    "situacao": "active",
    "responsavel": "John Doe",
    "observacoes": "Main application",
    "tags": ["prod"],
    "host_ids": [1, 2]
  }
]`}
          </pre>
        </details>

        <input
          ref={fileInputRef}
          type="file"
          accept=".json,application/json"
          onChange={handleFileSelect}
          className="hidden"
        />

        {!fileData ? (
          <button
            onClick={() => fileInputRef.current?.click()}
            className="w-full py-8 border-2 border-dashed border-[var(--border-default)] rounded-[var(--radius-lg)] hover:border-[var(--accent)] hover:bg-[var(--accent-muted)]/5 transition group"
          >
            <div className="flex flex-col items-center gap-2">
              <Icon path={ICON_PATHS.cloudUpload} className="w-8 h-8 text-[var(--text-faint)] group-hover:text-[var(--accent)] transition-colors" strokeWidth={1.5} />
              <span className="text-sm text-[var(--text-muted)] group-hover:text-[var(--text-secondary)]">
                {t("settings.importer.selectFile")}
              </span>
            </div>
          </button>
        ) : (
          <div className="space-y-3">
            <div className="flex items-center justify-between p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-default)]">
              <div className="flex items-center gap-2 min-w-0">
                <Icon path={ICON_PATHS.document} className="w-4 h-4 shrink-0 text-[var(--accent)]" />
                <span className="text-sm text-[var(--text-primary)] truncate font-mono">{fileName}</span>
              </div>
              <button onClick={reset} className="text-xs text-[var(--text-faint)] hover:text-[var(--danger)] transition-colors shrink-0 ml-2">
                {t("common.remove")}
              </button>
            </div>

            {/* Preview stats */}
            <div className="grid grid-cols-3 gap-3">
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-primary)] font-mono">{fileData.length}</div>
                <div className="text-2xs text-[var(--text-faint)]">{t("settings.importer.records")}</div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--accent)] font-mono">
                  {importType === "hosts"
                    ? fileData.filter(d => d.user || d.password).length
                    : fileData.filter(d => d.responsavel).length
                  }
                </div>
                <div className="text-2xs text-[var(--text-faint)]">
                  {importType === "hosts" ? t("settings.importer.withCreds") : t("settings.importer.withOwner")}
                </div>
              </div>
              <div className="p-3 rounded-[var(--radius-md)] bg-[var(--bg-elevated)] border border-[var(--border-subtle)] text-center">
                <div className="text-lg font-bold text-[var(--text-secondary)] font-mono">
                  {new Set(fileData.flatMap(d => (d.tags as string[]) || [])).size}
                </div>
                <div className="text-2xs text-[var(--text-faint)]">{t("common.tags")}</div>
              </div>
            </div>

            {detectedType && detectedType !== importType && (
              <div className="p-2.5 rounded-[var(--radius-md)] bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-[var(--warning)] text-xs">
                {t("settings.importer.typeMismatch", { detected: detectedType, current: importType })}
                <button onClick={() => setImportType(detectedType as "hosts" | "dns")} className="ml-1 underline hover:text-[var(--warning)]">
                  {t("settings.importer.switchTo", { type: detectedType })}
                </button>
              </div>
            )}
          </div>
        )}

        {parseError && <FormError message={parseError} />}
      </SectionCard>
      </div>

      {/* Import button + results */}
      {fileData && !result && (
        <div className="animate-slide-up" style={{ animationFillMode: "both" }}>
          <Button onClick={handleImport} loading={importing} className="w-full">
            {t("settings.importer.importButton", { count: String(fileData.length), type: importType === "hosts" ? t("nav.hosts") : t("settings.importer.dnsRecords") })}
          </Button>
        </div>
      )}

      {result && (
        <div className="animate-slide-up" style={{ animationFillMode: "both" }}>
        <SectionCard
          as="h3"
          title={t("settings.importer.results")}
          footer={<Button size="sm" variant="secondary" onClick={reset}>{t("settings.importer.another")}</Button>}
        >
          <div className="grid grid-cols-3 gap-3 mb-4">
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--success)]/10 border border-[var(--success)]/25 text-center">
              <div className="text-lg font-bold text-[var(--success)] font-mono">{result.created}</div>
              <div className="text-2xs text-[var(--success)]/70">{t("settings.importer.created")}</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--warning)]/10 border border-[var(--warning)]/25 text-center">
              <div className="text-lg font-bold text-[var(--warning)] font-mono">{result.skipped}</div>
              <div className="text-2xs text-[var(--warning)]/70">{t("settings.importer.skipped")}</div>
            </div>
            <div className="p-3 rounded-[var(--radius-md)] bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-center">
              <div className="text-lg font-bold text-[var(--danger)] font-mono">{result.failed}</div>
              <div className="text-2xs text-[var(--danger)]/70">{t("settings.importer.failed")}</div>
            </div>
          </div>

          {result.errors && result.errors.length > 0 && (
            <div className="max-h-48 overflow-y-auto space-y-1">
              {result.errors.map((err, i) => (
                <div key={i} className="flex items-start gap-2 text-xs p-2 rounded-[var(--radius-sm)] bg-[var(--bg-elevated)]">
                  <span className="text-[var(--text-faint)] shrink-0 tabular-nums font-mono">#{err.index}</span>
                  <span className="text-[var(--text-secondary)] truncate font-mono">{err.name}</span>
                  <span className="text-[var(--text-faint)] shrink-0">{err.error}</span>
                </div>
              ))}
            </div>
          )}
        </SectionCard>
        </div>
      )}
    </div>
  );
}
