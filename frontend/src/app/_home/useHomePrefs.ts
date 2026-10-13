"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authAPI } from "@/lib/api";
import { useFlag } from "@/contexts/FlagContext";
import { useLocale } from "@/contexts/LocaleContext";
import { readPrefs, type HomePrefs } from "./homeModel";

const KEY = ["home-prefs"];

/** The signed-in user's home preferences (users.home_prefs). `prefs` is null
 *  until onboarding is done. Saves are optimistic and roll back on failure. */
export function useHomePrefs() {
  const qc = useQueryClient();
  const flag = useFlag();
  const { t } = useLocale();
  const query = useQuery({ queryKey: KEY, queryFn: authAPI.homePrefs, staleTime: Infinity });
  const mutation = useMutation({
    mutationFn: (p: HomePrefs) => authAPI.setHomePrefs(p),
    onMutate: async (p) => {
      await qc.cancelQueries({ queryKey: KEY });
      const prev = qc.getQueryData(KEY);
      qc.setQueryData(KEY, p);
      return { prev };
    },
    onError: (err, _p, ctx) => {
      qc.setQueryData(KEY, ctx?.prev);
      flag({ appearance: "error", title: t("home.saveFailed"), description: err.message });
    },
  });
  return {
    loading: query.isLoading,
    error: query.error,
    prefs: readPrefs(query.data),
    save: mutation.mutate,
  };
}
