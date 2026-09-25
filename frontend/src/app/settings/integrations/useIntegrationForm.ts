"use client";

import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { integrationsAPI } from "@/lib/api";
import { useFlag } from "@/contexts/FlagContext";
import { useLocale } from "@/contexts/LocaleContext";

/** What GET /api/settings/integrations shows for a secret that is stored. */
export const MASKED = "••••••••";

/**
 * One integration group's settings form: the stored values with the admin's
 * edits layered on top. Only `edits` are ever PUT, so an untouched secret is
 * never sent, and a refetch (another section saving, window refocus) can't
 * overwrite what the admin is typing — nothing copies server state into state.
 */
export function useIntegrationForm(group: string) {
  const queryClient = useQueryClient();
  const flag = useFlag();
  const { t } = useLocale();
  const { data } = useQuery({ queryKey: ["integrations"], queryFn: integrationsAPI.get });
  const stored = data?.[group] ?? {};
  const [edits, setEdits] = useState<Record<string, string>>({});
  const form: Record<string, string> = { ...stored, ...edits };

  const drop = (key: string) =>
    setEdits((e) => Object.fromEntries(Object.entries(e).filter(([k]) => k !== key)));
  // Typing a value back to what is stored is not a change.
  const set = (key: string, value: string) =>
    value === (stored[key] ?? "") ? drop(key) : setEdits((e) => ({ ...e, [key]: value }));

  // Edits are cleared only after the refetch lands, so the form never flashes
  // the old stored values in between.
  const refetch = () => queryClient.invalidateQueries({ queryKey: ["integrations"] });
  const save = useMutation({
    mutationFn: (values: Record<string, string>) => integrationsAPI.update(group, values),
    onSuccess: async () => {
      await refetch();
      setEdits({});
      flag({ appearance: "success", title: t("settings.integrations.saved") });
    },
  });
  const clearSecret = useMutation({
    mutationFn: (key: string) => integrationsAPI.clearSecret(group, key),
    onSuccess: async (_res, key) => {
      await refetch();
      drop(key);
    },
  });

  return {
    data,
    form,
    set,
    dirty: Object.keys(edits).length > 0,
    save: () => save.mutate(edits),
    saving: save.isPending,
    saveError: save.error,
    saved: save.isSuccess,
    reset: () => {
      setEdits({});
      save.reset();
    },
    clearSecret: (key: string) => clearSecret.mutate(key),
    /** The server holds a value for this secret (it comes back masked). */
    isStored: (key: string) => stored[key] === MASKED,
  };
}

export type IntegrationForm = ReturnType<typeof useIntegrationForm>;
