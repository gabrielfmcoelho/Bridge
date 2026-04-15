"use client";

import { useCallback } from "react";
import type { HostChamado } from "@/lib/types";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import IconButton from "@/components/ui/IconButton";
import Button from "@/components/ui/Button";

function applyDateMask(raw: string): string {
  const digits = raw.replace(/\D/g, "").slice(0, 8);
  if (digits.length <= 2) return digits;
  if (digits.length <= 4) return `${digits.slice(0, 2)}/${digits.slice(2)}`;
  return `${digits.slice(0, 2)}/${digits.slice(2, 4)}/${digits.slice(4)}`;
}

function isValidDate(value: string): boolean {
  if (!/^\d{2}\/\d{2}\/\d{4}$/.test(value)) return false;
  const [dd, mm, yyyy] = value.split("/").map(Number);
  const d = new Date(yyyy, mm - 1, dd);
  return d.getFullYear() === yyyy && d.getMonth() === mm - 1 && d.getDate() === dd;
}

interface ChamadoListProps {
  value: HostChamado[];
  onChange: (v: HostChamado[]) => void;
  users: { id: number; display_name: string }[];
  t: (k: string) => string;
}

export default function ChamadoList({
  value,
  onChange,
  users,
  t,
}: ChamadoListProps) {
  const userOptions = users.map((u) => ({
    value: String(u.id),
    label: u.display_name,
  }));

  const update = (idx: number, patch: Partial<HostChamado>) => {
    const next = value.map((item, i) => (i === idx ? { ...item, ...patch } : item));
    onChange(next);
  };

  const remove = (idx: number) => {
    onChange(value.filter((_, i) => i !== idx));
  };

  const add = () => {
    const defaultUserId = users.length > 0 ? users[0].id : 0;
    onChange([
      ...value,
      {
        chamado_id: "",
        title: "",
        status: "in_execution",
        user_id: defaultUserId,
        user_display_name: users.length > 0 ? users[0].display_name : "",
        date: "",
      },
    ]);
  };

  return (
    <div className="space-y-3">
      {value.map((item, idx) => (
        <div key={idx} className="relative pb-3 mb-3 border-b border-[var(--border-subtle)] last:border-0 last:mb-0 last:pb-0">
          <div className="absolute top-0 right-0">
            <IconButton
              variant="danger"
              size="sm"
              type="button"
              onClick={() => remove(idx)}
              title={t("common.remove")}
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </IconButton>
          </div>

          <div className="space-y-3 pr-8">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <Input
                label={t("chamado.id")}
                value={item.chamado_id}
                onChange={(e) => update(idx, { chamado_id: e.target.value })}
                placeholder="GLPI #..."
              />
              <Input
                label={t("common.title")}
                value={item.title || ""}
                onChange={(e) => update(idx, { title: e.target.value })}
                placeholder={t("chamado.titlePlaceholder")}
              />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
              <Select
                label={t("common.status") || "Status"}
                value={item.status || "in_execution"}
                options={[
                  { value: "in_execution", label: t("chamado.inExecution") || "In Execution" },
                  { value: "solved", label: t("chamado.solved") || "Solved" },
                ]}
                onChange={(e) => update(idx, { status: e.target.value })}
              />
              <Select
                label={t("host.chamadoUser")}
                value={String(item.user_id)}
                options={userOptions}
                onChange={(e) => {
                  const uid = Number(e.target.value);
                  const user = users.find((u) => u.id === uid);
                  update(idx, {
                    user_id: uid,
                    user_display_name: user?.display_name || "",
                  });
                }}
              />
              <Input
                label={t("host.chamadoDate")}
                value={item.date}
                onChange={(e) => update(idx, { date: applyDateMask(e.target.value) })}
                placeholder="DD/MM/YYYY"
                maxLength={10}
                error={item.date.length === 10 && !isValidDate(item.date) ? "Data inv\u00e1lida" : undefined}
              />
            </div>
          </div>
        </div>
      ))}

      <Button type="button" variant="secondary" size="sm" onClick={add}>
        <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
        </svg>
        {t("chamado.add")}
      </Button>
    </div>
  );
}
