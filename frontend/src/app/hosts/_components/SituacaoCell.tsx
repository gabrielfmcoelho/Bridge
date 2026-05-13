"use client";

import { useState } from "react";
import * as Popover from "@radix-ui/react-popover";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import Badge from "@/components/ui/Badge";
import { hostsAPI, enumsAPI } from "@/lib/api";
import type { Host, EnumOption } from "@/lib/types";

// SituacaoCell renders the host's situação badge inline in a table cell.
// When canEdit=true, clicking the badge opens a popover that lets the
// operator change the situação without leaving the row. Submission goes
// through hostsAPI.update and refreshes the host list query on success.
//
// The trigger explicitly stops click propagation so the parent row's
// onClick (which navigates to the host detail page) doesn't fire when
// the user is just trying to edit the badge.
export default function SituacaoCell({
  host,
  canEdit,
  t,
}: {
  host: Host;
  canEdit: boolean;
  t: (key: string) => string;
}) {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  const { data: situacoes } = useQuery({
    queryKey: ["enums", "situacao"],
    queryFn: () => enumsAPI.list("situacao"),
    enabled: canEdit && open,
  });

  const mutation = useMutation({
    mutationFn: (situacao: string) =>
      hostsAPI.update(host.oficial_slug, { situacao }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
      queryClient.invalidateQueries({ queryKey: ["dashboard"] });
      setOpen(false);
    },
  });

  if (!canEdit) {
    return (
      <Badge variant="situacao" situacao={host.situacao} dot>
        {host.situacao}
      </Badge>
    );
  }

  const options: EnumOption[] = Array.isArray(situacoes) ? situacoes : [];

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          onClick={(e) => {
            // Block the parent row's navigation onClick. Without this,
            // clicking the badge would race the popover open against a
            // page transition and lose.
            e.stopPropagation();
          }}
          className="cursor-pointer hover:ring-2 hover:ring-[var(--accent-muted)] rounded-full transition-shadow"
          title={t("host.changeSituacao")}
        >
          <Badge variant="situacao" situacao={host.situacao} dot>
            {host.situacao}
          </Badge>
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          side="bottom"
          align="start"
          sideOffset={4}
          className="bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg p-1 min-w-[160px] z-50"
          onClick={(e) => e.stopPropagation()}
        >
          {options.length === 0 && (
            <div className="px-2 py-1.5 text-xs text-[var(--text-faint)]">
              {t("common.loading")}
            </div>
          )}
          {options.map((opt) => {
            const isCurrent = opt.value === host.situacao;
            return (
              <button
                key={opt.value}
                type="button"
                onClick={(e) => {
                  e.stopPropagation();
                  if (isCurrent) {
                    setOpen(false);
                    return;
                  }
                  mutation.mutate(opt.value);
                }}
                disabled={mutation.isPending}
                className={`w-full flex items-center justify-between gap-2 text-left px-2 py-1.5 rounded text-xs transition-colors hover:bg-[var(--bg-elevated)] disabled:opacity-50 ${
                  isCurrent ? "font-semibold text-[var(--text-primary)]" : "text-[var(--text-secondary)]"
                }`}
              >
                <span className="flex items-center gap-2">
                  <span
                    className="w-2 h-2 rounded-full shrink-0"
                    style={{ backgroundColor: opt.color || "var(--text-faint)" }}
                  />
                  {opt.value}
                </span>
                {isCurrent && <span className="text-[var(--accent)]">✓</span>}
              </button>
            );
          })}
          {mutation.isError && (
            <div className="px-2 py-1.5 text-[10px] text-red-400">
              {(mutation.error instanceof Error ? mutation.error.message : null) ?? "Update failed"}
            </div>
          )}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
