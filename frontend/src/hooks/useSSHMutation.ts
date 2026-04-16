import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";

type ConsoleStatus = "success" | "error" | "warning" | "loading";

interface UseSSHMutationOptions<TData, TVariables> {
  slug: string;
  mutationFn: (vars: TVariables) => Promise<TData>;
  /** Called to build the console entry on success. Return null to skip pushConsole. */
  onResult: (data: TData) => { status: ConsoleStatus; content: ReactNode } | null;
  /** Translation key used as the console label (e.g. "operation.repairDevNull"). */
  label: string;
  /** Push a console entry. Provided by the parent component. */
  pushConsole: (label: string, status: ConsoleStatus, content: ReactNode) => void;
  /** Additional invalidation query keys beyond the defaults. */
  extraInvalidateKeys?: string[][];
  /** Called after invalidation on success, before pushConsole. */
  onAfterSuccess?: (data: TData) => void;
}

/**
 * Wraps useMutation with the standard SSH operation boilerplate:
 * - invalidates host, hosts, and operation-logs queries
 * - pushes success/error to the console drawer
 * - handles the common error pattern
 */
export function useSSHMutation<TData, TVariables = void>({
  slug,
  mutationFn,
  onResult,
  label,
  pushConsole,
  extraInvalidateKeys,
  onAfterSuccess,
}: UseSSHMutationOptions<TData, TVariables>) {
  const queryClient = useQueryClient();

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ["host", slug] });
    queryClient.invalidateQueries({ queryKey: ["hosts"] });
    queryClient.invalidateQueries({ queryKey: ["operation-logs", slug] });
    extraInvalidateKeys?.forEach((key) => {
      queryClient.invalidateQueries({ queryKey: key });
    });
  };

  return useMutation({
    mutationFn,
    onSuccess: (data) => {
      invalidateAll();
      onAfterSuccess?.(data);
      const entry = onResult(data);
      if (entry) {
        pushConsole(label, entry.status, entry.content);
      }
    },
    onError: (err) => {
      invalidateAll();
      pushConsole(label, "error", err instanceof Error ? err.message : "Failed");
    },
  });
}
