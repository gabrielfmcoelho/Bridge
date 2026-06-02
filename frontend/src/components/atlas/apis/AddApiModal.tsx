"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import Select from "@/components/ui/Select";
import FormError from "@/components/ui/FormError";
import { apiCatalogAPI, projectsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalog } from "@/lib/types";

type Tab = "upload" | "url";

export default function AddApiModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (api: ApiCatalog) => void;
}) {
  const { t } = useLocale();
  const [tab, setTab] = useState<Tab>("upload");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [scope, setScope] = useState("avulso");
  const [parentId, setParentId] = useState<string>("");
  const [file, setFile] = useState<File | null>(null);
  const [sourceUrl, setSourceUrl] = useState("");
  const [baseUrl, setBaseUrl] = useState("");
  const [docsUrl, setDocsUrl] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const { data: projects = [] } = useQuery({
    queryKey: ["projects"],
    queryFn: () => projectsAPI.list(),
    enabled: open && scope === "projeto",
  });

  function reset() {
    setTab("upload");
    setName("");
    setDescription("");
    setScope("avulso");
    setParentId("");
    setFile(null);
    setSourceUrl("");
    setBaseUrl("");
    setDocsUrl("");
    setError(null);
    setSubmitting(false);
  }

  function close() {
    reset();
    onClose();
  }

  async function submit() {
    setError(null);
    if (scope === "projeto" && !parentId) {
      setError(t("atlas.apis.selectProject"));
      return;
    }
    const meta = {
      name: name.trim() || undefined,
      description: description.trim() || undefined,
      scope,
      parent_id: scope === "projeto" ? Number(parentId) : undefined,
      base_url: baseUrl.trim() || undefined,
      docs_url: docsUrl.trim() || undefined,
    };
    setSubmitting(true);
    try {
      let created: ApiCatalog;
      if (tab === "upload") {
        if (!file) {
          setError(t("atlas.apis.specFile"));
          setSubmitting(false);
          return;
        }
        created = await apiCatalogAPI.importUpload(file, meta);
      } else {
        if (!sourceUrl.trim()) {
          setError(t("atlas.apis.sourceUrl"));
          setSubmitting(false);
          return;
        }
        created = await apiCatalogAPI.importURL({ ...meta, source_url: sourceUrl.trim() });
      }
      onCreated(created);
      close();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setSubmitting(false);
    }
  }

  const projectOptions = [
    { value: "", label: t("atlas.apis.selectProject") },
    ...projects.map((p) => ({ value: String(p.id), label: p.name })),
  ];

  return (
    <Modal open={open} onClose={close} title={t("atlas.apis.importTitle")}>
      <div className="space-y-4">
        {/* Source tabs */}
        <div className="flex gap-2">
          {(["upload", "url"] as Tab[]).map((tk) => (
            <button
              key={tk}
              type="button"
              onClick={() => setTab(tk)}
              className={`px-3 py-1.5 text-sm rounded-[var(--radius-md)] border ${
                tab === tk
                  ? "border-[var(--accent)] text-[var(--accent)] bg-[var(--accent)]/10"
                  : "border-[var(--border-default)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              {tk === "upload" ? t("atlas.apis.importUpload") : t("atlas.apis.importUrl")}
            </button>
          ))}
        </div>

        {tab === "upload" ? (
          <div>
            <label className="block text-sm mb-1 text-[var(--text-secondary)]">{t("atlas.apis.specFile")}</label>
            <input
              type="file"
              accept=".json,.yaml,.yml,application/json,application/yaml,text/yaml"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
              className="block w-full text-sm text-[var(--text-secondary)] file:mr-3 file:py-1.5 file:px-3 file:rounded-[var(--radius-md)] file:border file:border-[var(--border-default)] file:bg-[var(--bg-elevated)] file:text-[var(--text-secondary)]"
            />
          </div>
        ) : (
          <div>
            <Input
              label={t("atlas.apis.sourceUrl")}
              value={sourceUrl}
              onChange={(e) => setSourceUrl(e.target.value)}
              placeholder="https://api.example.com/openapi.json"
            />
            <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("atlas.apis.sourceUrlHint")}</p>
          </div>
        )}

        {/* Base + Docs apply to both upload and URL imports. */}
        <div>
          <Input
            label={t("atlas.apis.baseUrl")}
            value={baseUrl}
            onChange={(e) => setBaseUrl(e.target.value)}
            placeholder="https://api.example.com"
          />
          <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("atlas.apis.baseUrlHint")}</p>
        </div>
        <div>
          <Input
            label={t("atlas.apis.docsUrl")}
            value={docsUrl}
            onChange={(e) => setDocsUrl(e.target.value)}
            placeholder="https://api.example.com/docs"
          />
          <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("atlas.apis.docsUrlHint")}</p>
        </div>

        <Input
          label={t("atlas.apis.name")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t("atlas.apis.namePlaceholder")}
        />
        <Textarea
          label={t("atlas.apis.description")}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
        />
        <Select
          label={t("atlas.apis.scope")}
          value={scope}
          onChange={(e) => setScope(e.target.value)}
          options={[
            { value: "avulso", label: t("atlas.apis.scopeAvulso") },
            { value: "projeto", label: t("atlas.apis.scopeProjeto") },
          ]}
        />
        {scope === "projeto" && (
          <Select
            label={t("atlas.apis.project")}
            value={parentId}
            onChange={(e) => setParentId(e.target.value)}
            options={projectOptions}
          />
        )}

        {error && <FormError message={error} />}

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={close} type="button">
            {t("common.cancel") || "Cancel"}
          </Button>
          <Button onClick={submit} loading={submitting} type="button">
            {submitting ? t("atlas.apis.importing") : t("atlas.apis.import")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
