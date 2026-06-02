"use client";

import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import Modal from "@/components/ui/Modal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Textarea from "@/components/ui/Textarea";
import FormError from "@/components/ui/FormError";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ApiCatalog } from "@/lib/types";

// EditApiModal edits a cataloged API's metadata: name, description, and the
// Base URL (API host for Test Request) + Docs URL (open-externally target).
// The spec document itself is changed via Refetch / re-import, not here.
export default function EditApiModal({
  open,
  onClose,
  api,
}: {
  open: boolean;
  onClose: () => void;
  api: ApiCatalog;
}) {
  const { t } = useLocale();
  const qc = useQueryClient();
  const [name, setName] = useState(api.name);
  const [description, setDescription] = useState(api.description ?? "");
  const [baseUrl, setBaseUrl] = useState(api.base_url ?? "");
  const [docsUrl, setDocsUrl] = useState(api.docs_url ?? "");
  const [error, setError] = useState<string | null>(null);

  // Re-sync when opened for a (possibly different / refetched) api.
  /* eslint-disable react-hooks/set-state-in-effect -- syncing form fields from
     the api prop when the modal opens; the external→state sync the rule flags. */
  useEffect(() => {
    if (!open) return;
    setName(api.name);
    setDescription(api.description ?? "");
    setBaseUrl(api.base_url ?? "");
    setDocsUrl(api.docs_url ?? "");
    setError(null);
  }, [open, api]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const save = useMutation({
    mutationFn: () =>
      apiCatalogAPI.update(api.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        base_url: baseUrl.trim() || undefined,
        docs_url: docsUrl.trim() || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["api-catalog", api.id] });
      qc.invalidateQueries({ queryKey: ["api-catalog"] });
      onClose();
    },
    onError: (e) => setError(e instanceof Error ? e.message : String(e)),
  });

  return (
    <Modal open={open} onClose={onClose} title={t("atlas.apis.editTitle")}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault();
          setError(null);
          if (!name.trim()) {
            setError(t("atlas.apis.name"));
            return;
          }
          save.mutate();
        }}
      >
        <Input label={t("atlas.apis.name")} value={name} onChange={(e) => setName(e.target.value)} />
        <Textarea label={t("atlas.apis.description")} value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        <div>
          <Input label={t("atlas.apis.baseUrl")} value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="https://api.example.com" />
          <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("atlas.apis.baseUrlHint")}</p>
        </div>
        <div>
          <Input label={t("atlas.apis.docsUrl")} value={docsUrl} onChange={(e) => setDocsUrl(e.target.value)} placeholder="https://api.example.com/docs" />
          <p className="text-[10px] text-[var(--text-faint)] mt-1">{t("atlas.apis.docsUrlHint")}</p>
        </div>

        {error && <FormError message={error} />}

        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" type="button" onClick={onClose}>
            {t("common.cancel") || "Cancel"}
          </Button>
          <Button type="submit" loading={save.isPending}>
            {t("atlas.apis.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
