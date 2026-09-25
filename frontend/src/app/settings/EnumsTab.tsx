"use client";

import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Badge from "@/components/ui/Badge";
import ResponsiveModal from "@/components/ui/ResponsiveModal";

export default function EnumsTab() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const [newValue, setNewValue] = useState<Record<string, string>>({});
  const [newColor, setNewColor] = useState<Record<string, string>>({});
  const [editing, setEditing] = useState<{ category: string; value: string } | null>(null);
  const [editValue, setEditValue] = useState("");
  const [editColor, setEditColor] = useState("");
  const [editError, setEditError] = useState("");
  const [newCategory, setNewCategory] = useState("");
  const [newCategoryValue, setNewCategoryValue] = useState("");
  const isAdmin = user?.role === "admin";

  const { data: allEnums = {} } = useQuery({
    queryKey: ["enums"],
    queryFn: enumsAPI.listAll,
  });

  const addMutation = useMutation({
    mutationFn: ({ category, value, color }: { category: string; value: string; color?: string }) =>
      enumsAPI.create(category, value, color),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["enums"] }),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ category, value }: { category: string; value: string }) =>
      enumsAPI.delete(category, value),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["enums"] }),
  });

  const updateMutation = useMutation({
    mutationFn: () => {
      if (!editing) throw new Error(t("settings.enumsNoItemSelected"));
      return enumsAPI.update(editing.category, editing.value, editValue, editColor);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["enums"] });
      setEditing(null);
      setEditError("");
    },
    onError: (err) => setEditError(err instanceof Error ? err.message : t("common.requestFailed")),
  });

  const openEdit = (category: string, value: string) => {
    setEditing({ category, value });
    setEditValue(value);
    const option = (allEnums[category] || []).find((opt) => opt.value === value);
    setEditColor(option?.color || "");
    setEditError("");
  };

  const isSituacaoCategory = (category: string) => category === "situacao";

  // System categories that must always appear even if empty
  const REQUIRED_CATEGORIES = [
    "hospedagem", "situacao", "tipo_maquina", "orchestrator_type",
    "entidade_responsavel", "issue_status", "issue_priority",
  ];
  const mergedEnums = { ...allEnums };
  for (const cat of REQUIRED_CATEGORIES) {
    if (!mergedEnums[cat]) mergedEnums[cat] = [];
  }

  return (
    <div className="space-y-4">
      {/* One list, not one card per enum: these categories are peers, and a
          card per peer spent a full screen on five one-input rows. */}
      <SectionCard as="h3" title={t("settings.enums")} count={Object.keys(mergedEnums).length} body="flush">
        <ul className="divide-y divide-[var(--border-subtle)]">
      {Object.entries(mergedEnums).map(([category, options], i) => (
        <li key={category} className="px-4 py-3 stagger-in" style={{ "--i": i } as React.CSSProperties}>
          <h3 className="text-xs font-semibold mb-2 font-mono text-[var(--text-secondary)]">
            {category}
          </h3>
          <div className="flex flex-wrap gap-1.5 mb-2">
            {options.map((opt) => (
              <span key={opt.value} className="group inline-flex items-center gap-1">
                <button type="button" onClick={() => isAdmin && openEdit(category, opt.value)} className={isAdmin ? "cursor-pointer" : ""}>
                  <Badge>
                    {isSituacaoCategory(category) && opt.color && (
                      <span className="w-2 h-2 rounded-full border border-white/20" style={{ backgroundColor: opt.color }} />
                    )}
                    {opt.value}
                  </Badge>
                </button>
                {isAdmin && (
                  <button
                    onClick={() => deleteMutation.mutate({ category, value: opt.value })}
                    className="opacity-0 group-hover:opacity-100 text-[var(--text-faint)] hover:text-[var(--danger)] text-xs transition-opacity"
                  >
                    &times;
                  </button>
                )}
              </span>
            ))}
            {options.length === 0 && (
              <span className="text-xs text-[var(--text-faint)]">{t("settings.enumsNoValues")}</span>
            )}
          </div>
          {isAdmin && (
            <form className="flex gap-2" onSubmit={(e) => {
              e.preventDefault();
              if (newValue[category]) {
                addMutation.mutate({
                  category,
                  value: newValue[category],
                  color: isSituacaoCategory(category) ? (newColor[category] || "") : "",
                });
                setNewValue((v) => ({ ...v, [category]: "" }));
                setNewColor((v) => ({ ...v, [category]: "" }));
              }
            }}>
              <Input
                value={newValue[category] || ""}
                onChange={(e) => setNewValue((v) => ({ ...v, [category]: e.target.value }))}
                placeholder={t("settings.enumsNewValuePlaceholder", { category })}
                className="max-w-xs"
              />
              <Button size="sm" type="submit">
                {t("common.add")}
              </Button>
              {isSituacaoCategory(category) && (
                <div className="flex items-end gap-2">
                  <Input
                    type="color"
                    value={newColor[category] || "#06b6d4"}
                    onChange={(e) => setNewColor((v) => ({ ...v, [category]: e.target.value }))}
                    className="w-12 h-9 p-1"
                    title={t("settings.enumsStatusColorTitle")}
                  />
                </div>
              )}
            </form>
          )}
        </li>
      ))}
        </ul>
      </SectionCard>

      {/* Add new category */}
      {isAdmin && (
        <SectionCard as="h3" title={t("settings.enumsNewCategory")}>
          <form className="flex gap-2 flex-wrap" onSubmit={(e) => {
            e.preventDefault();
            if (newCategory && newCategoryValue) {
              addMutation.mutate({ category: newCategory, value: newCategoryValue, color: newCategory === "situacao" ? "#06b6d4" : "" });
              setNewCategory("");
              setNewCategoryValue("");
            }
          }}>
            <Input
              value={newCategory}
              onChange={(e) => setNewCategory(e.target.value)}
              placeholder={t("settings.enumsCategoryNamePlaceholder")}
              className="max-w-[180px]"
            />
            <Input
              value={newCategoryValue}
              onChange={(e) => setNewCategoryValue(e.target.value)}
              placeholder={t("settings.enumsFirstValuePlaceholder")}
              className="max-w-[180px]"
            />
            <Button size="sm" type="submit">
              {t("common.create")}
            </Button>
          </form>
        </SectionCard>
      )}

      {/* Edit enum value modal */}
      <ResponsiveModal open={editing !== null} onClose={() => setEditing(null)} title={`${t("common.edit")} — ${editing?.category}`}>
        {editing && (
          <form onSubmit={(e) => { e.preventDefault(); updateMutation.mutate(); }} className="space-y-4">
            {editError && (
              <div className="bg-[var(--danger)]/10 border border-[var(--danger)]/25 text-[var(--danger)] text-sm rounded-[var(--radius-md)] p-3 animate-slide-down">{editError}</div>
            )}
            <Input
              label={t("settings.enumsValueLabel")}
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              required
            />
            {editing.category === "situacao" && (
              <Input
                label={t("settings.enumsColorLabel")}
                type="color"
                value={editColor || "#06b6d4"}
                onChange={(e) => setEditColor(e.target.value)}
                className="w-16 h-10 p-1"
              />
            )}
            <div className="flex justify-end gap-2">
              <Button type="button" variant="secondary" onClick={() => setEditing(null)}>
                {t("common.cancel")}
              </Button>
              <Button type="submit" loading={updateMutation.isPending}>
                {t("common.save")}
              </Button>
            </div>
          </form>
        )}
      </ResponsiveModal>
    </div>
  );
}
