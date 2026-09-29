"use client";

import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { projectEmbedsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useConfirm } from "@/contexts/ConfirmContext";
import Input from "@/components/ui/Input";
import FormError from "@/components/ui/FormError";
import FormFooter from "@/components/ui/FormFooter";
import Button from "@/components/ui/Button";
import type { ProjectEmbed } from "@/lib/types";

const URL_RE = /^https?:\/\/\S+$/i;

/** Add/edit a BI or tool the project shows in an iframe. */
export default function EmbedForm({ projectId, embed, onDone, onCancel }: {
  projectId: number;
  embed?: ProjectEmbed;
  onDone: () => void;
  onCancel: () => void;
}) {
  const { t } = useLocale();
  const confirm = useConfirm();
  const [title, setTitle] = useState(embed?.title ?? "");
  const [url, setUrl] = useState(embed?.url ?? "");
  const [height, setHeight] = useState(String(embed?.height ?? 600));
  const [error, setError] = useState("");
  const [attempted, setAttempted] = useState(false);

  const onError = (err: unknown) => setError(err instanceof Error ? err.message : t("form.saveFailed"));
  const save = useMutation({
    mutationFn: () => {
      const payload = { title: title.trim(), url: url.trim(), height: parseInt(height, 10) || 600, sort_order: embed?.sort_order ?? 0 };
      return embed ? projectEmbedsAPI.update(projectId, embed.id, payload) : projectEmbedsAPI.create(projectId, payload);
    },
    onSuccess: onDone,
    onError,
  });
  const remove = useMutation({ mutationFn: () => projectEmbedsAPI.delete(projectId, embed!.id), onSuccess: onDone, onError });

  const errors: Record<string, string> = {};
  if (!title.trim()) errors.title = t("form.required");
  if (!URL_RE.test(url.trim())) errors.url = t("form.urlInvalid");
  const err = (k: string) => (attempted ? errors[k] : undefined);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    setAttempted(true);
    if (Object.keys(errors).length) return;
    setError("");
    save.mutate();
  };

  return (
    <form onSubmit={submit} className="space-y-4" noValidate>
      <FormError message={error} />
      <Input label={t("embed.titleField")} value={title} onChange={(e) => setTitle(e.target.value)} required autoFocus
        placeholder={t("embed.titlePlaceholder")} error={err("title")} aria-invalid={!!err("title")} />
      <Input label="URL" type="url" value={url} onChange={(e) => setUrl(e.target.value)} required
        placeholder="https://" hint={t("embed.urlHint")} error={err("url")} aria-invalid={!!err("url")} />
      <Input label={t("embed.height")} type="number" min={200} step={50} value={height} onChange={(e) => setHeight(e.target.value)} />
      <FormFooter
        onCancel={onCancel}
        submitType="submit"
        submitLabel={embed ? t("form.saveChanges") : t("embed.add")}
        loading={save.isPending}
        start={embed ? (
          <Button type="button" size="sm" variant="ghost" loading={remove.isPending}
            onClick={async () => { if (await confirm({ title: t("embed.deleteConfirm", { name: embed.title }), danger: true, confirmLabel: t("common.delete") })) remove.mutate(); }}>
            {t("common.delete")}
          </Button>
        ) : undefined}
      />
    </form>
  );
}
