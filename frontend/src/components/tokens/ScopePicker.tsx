"use client";

import { useQuery } from "@tanstack/react-query";
import type { ApiKeyScope } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import FormField from "@/components/ui/FormField";
import Checkbox from "@/components/ui/Checkbox";
import StatusAlert from "@/components/ui/StatusAlert";
import { Skeleton } from "@/components/ui/Skeleton";

// Picks what a new key or token may reach, from a scope catalogue: a SEAD
// service's (GET /admin/keys/scopes, via Bridge) or Bridge's own token scopes.
// "*" is full access and excludes the route scopes; modifiers ("demo")
// combine with either. isUsable greys out what the owner couldn't use anyway.
// The server enforces all of it.
export default function ScopePicker({ queryKey, load, value, onChange, error, isUsable }: {
  queryKey: unknown[];
  load: () => Promise<ApiKeyScope[]>;
  value: string[];
  onChange: (scopes: string[]) => void;
  error?: string;
  isUsable?: (scope: ApiKeyScope) => boolean;
}) {
  const { t } = useLocale();
  const { data: catalogue = [], isLoading, error: loadError } = useQuery({
    queryKey,
    queryFn: load,
    retry: false,
    staleTime: 5 * 60_000,
  });

  const has = (name: string) => value.includes(name);
  const toggle = (s: ApiKeyScope, on: boolean) => {
    if (!on) return onChange(value.filter((v) => v !== s.name));
    const modifiers = value.filter((v) => catalogue.find((c) => c.name === v)?.kind === "modifier");
    if (s.kind === "wildcard") return onChange(["*", ...modifiers]);
    if (s.kind === "route") return onChange([...value.filter((v) => v !== "*"), s.name]);
    onChange([...value, s.name]);
  };

  const groups: { kind: ApiKeyScope["kind"]; title: string }[] = [
    { kind: "route", title: t("atlas.apis.keys.scopeGroup.route") },
    { kind: "modifier", title: t("atlas.apis.keys.scopeGroup.modifier") },
    { kind: "wildcard", title: t("atlas.apis.keys.scopeGroup.wildcard") },
  ];

  return (
    <FormField label={t("atlas.apis.keys.scopes")} hint={t("atlas.apis.keys.scopesHint")} error={error} required>
      {isLoading ? (
        <Skeleton className="h-32 w-full rounded-[var(--radius-md)]" />
      ) : loadError ? (
        <StatusAlert variant="error">{t("atlas.apis.keys.scopesLoadFailed", { error: (loadError as Error).message })}</StatusAlert>
      ) : (
        <div className="space-y-4" aria-invalid={!!error}>
          {groups.map((g) => {
            const items = catalogue.filter((s) => s.kind === g.kind);
            if (items.length === 0) return null;
            return (
              <fieldset key={g.kind} className="space-y-2">
                <legend className="text-xs font-medium text-[var(--text-muted)] mb-1">{g.title}</legend>
                {items.map((s) => (
                  <div key={s.name} className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] px-3 py-2">
                    <Checkbox
                      label={s.kind === "wildcard" ? t("atlas.apis.keys.scopeWildcard") : s.name}
                      checked={has(s.name)}
                      onChange={(on) => toggle(s, on)}
                      disabled={(s.kind === "route" && has("*")) || (isUsable ? !isUsable(s) : false)}
                    />
                    <p className="mt-1 ml-6 text-xs text-[var(--text-muted)]">{s.description}</p>
                    {s.routes.length > 0 && (
                      <p className="mt-1 ml-6 text-2xs font-mono text-[var(--text-muted)] break-all">
                        {s.routes.map((r) => (r.startsWith("=") ? r.slice(1) : `${r}/…`)).join("  ")}
                      </p>
                    )}
                  </div>
                ))}
              </fieldset>
            );
          })}
        </div>
      )}
    </FormField>
  );
}
