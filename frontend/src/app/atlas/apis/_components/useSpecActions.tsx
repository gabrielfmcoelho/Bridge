"use client";

import { useRef } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiCatalogAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import type { ApiCatalog } from "@/lib/types";

export const SPEC_ACCEPT = ".json,.yaml,.yml,application/json,application/yaml,text/yaml";

/**
 * Refresh an API's spec in place: re-fetch it from its source URL, or upload
 * a new file over it. Both keep the id, links and grants; the outcome is a flag.
 * Render `fileInput` once and call `pickFile()` from a button or menu item.
 */
export function useSpecActions(api: Pick<ApiCatalog, "id" | "name"> | undefined) {
  const { t } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const inputRef = useRef<HTMLInputElement>(null);

  const done = (a: ApiCatalog) => {
    qc.invalidateQueries({ queryKey: ["api-catalog"] });
    qc.invalidateQueries({ queryKey: ["api-ops"] });
    flag({ appearance: "success", title: t("atlas.apis.specUpdated"), description: t("atlas.apis.endpointsCount", { count: String(a.operation_count) }) });
  };
  const failed = (title: string) => (e: unknown) =>
    flag({ appearance: "error", title, description: e instanceof Error ? e.message : t("form.saveFailed") });

  const refetch = useMutation({
    mutationFn: () => apiCatalogAPI.refetch(api!.id),
    onSuccess: done,
    onError: failed(t("atlas.apis.refetch")),
  });
  const replace = useMutation({
    mutationFn: (file: File) => apiCatalogAPI.replaceSpec(api!.id, file),
    onSuccess: done,
    onError: failed(t("atlas.apis.replaceSpec")),
  });

  const fileInput = (
    <input
      ref={inputRef}
      type="file"
      accept={SPEC_ACCEPT}
      className="hidden"
      aria-label={t("atlas.apis.replaceSpec")}
      onChange={(e) => {
        const file = e.target.files?.[0];
        e.target.value = "";
        if (file && api) replace.mutate(file);
      }}
    />
  );

  return { refetch, replace, fileInput, pickFile: () => inputRef.current?.click(), pending: refetch.isPending || replace.isPending };
}
