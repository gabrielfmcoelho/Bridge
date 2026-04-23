"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import Button from "@/components/ui/Button";
import { Skeleton } from "@/components/ui/Skeleton";
import FormcreatorQuestion, {
  SUPPORTED_QUESTION_TYPES,
  ID_LIKE_FIELDTYPES,
} from "@/components/glpi/FormcreatorQuestion";
import {
  glpiAPI,
  type FormcreatorBundle,
  type FormcreatorCondition,
  type FormcreatorQuestion as Question,
  type FormcreatorSection,
} from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  formID: number | null;
  profileID: number | null;
}

// Coerce different value shapes to a comparable string for condition checks.
function valueToString(v: unknown): string {
  if (v == null) return "";
  if (Array.isArray(v)) return v.map((x) => String(x)).join(",");
  return String(v);
}

function evaluateOne(
  cond: FormcreatorCondition,
  answers: Record<string, unknown>
): boolean {
  const left = valueToString(answers[String(cond.plugin_formcreator_questions_id)]);
  const right = String(cond.show_value ?? "");
  switch (cond.show_condition) {
    case "eq":
      return left === right;
    case "neq":
      return left !== right;
    case "lt":
      return Number(left) < Number(right);
    case "le":
      return Number(left) <= Number(right);
    case "gt":
      return Number(left) > Number(right);
    case "ge":
      return Number(left) >= Number(right);
    case "regex":
      try {
        return new RegExp(right).test(left);
      } catch {
        return false;
      }
    default:
      return true;
  }
}

// isVisible: no conditions → visible; otherwise apply per-row evaluator with
// the show_logic (AND/OR) of the FIRST condition to combine results — that's
// how Formcreator itself joins them.
function isVisible(
  conditions: FormcreatorCondition[],
  answers: Record<string, unknown>
): boolean {
  if (conditions.length === 0) return true;
  const results = conditions.map((c) => evaluateOne(c, answers));
  const logic = (conditions[0].show_logic || "AND").toUpperCase();
  return logic === "OR" ? results.some(Boolean) : results.every(Boolean);
}

function isEmpty(v: unknown): boolean {
  if (v == null) return true;
  if (Array.isArray(v)) return v.length === 0;
  if (typeof v === "string") return v.trim().length === 0;
  return false;
}

// isEmptyForType folds in the "id = 0 means unset" rule for FK-style field
// types, so a required dropdown with default_values="0" flags as missing.
function isEmptyForType(fieldtype: string, v: unknown): boolean {
  if (isEmpty(v)) return true;
  if (!ID_LIKE_FIELDTYPES.has(fieldtype)) return false;
  const probe = (entry: unknown): boolean => {
    const s = String(entry ?? "");
    const pipe = s.indexOf("|");
    const idStr = pipe >= 0 ? s.slice(0, pipe) : s;
    const n = parseInt(idStr, 10);
    return !Number.isFinite(n) || n === 0;
  };
  if (Array.isArray(v)) {
    return v.every(probe); // empty if every item is 0/invalid
  }
  return probe(v);
}

// normalizeAnswer converts UI-shaped answer values into the plain shape
// Formcreator expects on the wire. Single-value pickers stash the display
// label next to the id as "<id>|<label>"; multi-value pickers (multi-actor,
// tag) hold arrays of those strings (tags may further append "#<hex>" for
// color). File questions hold an array of UploadedDoc — we reduce to the
// doc id list.
function normalizeAnswer(fieldtype: string, value: unknown): unknown {
  const extractID = (entry: unknown): number | null => {
    if (entry == null || entry === "") return null;
    const s = String(entry);
    const pipe = s.indexOf("|");
    const idStr = pipe >= 0 ? s.slice(0, pipe) : s;
    const id = parseInt(idStr, 10);
    // 0 is GLPI's "unset" sentinel — treat it as no selection rather than
    // forwarding a bogus FK reference that triggers ERROR_GLPI_ADD.
    return Number.isFinite(id) && id > 0 ? id : null;
  };

  switch (fieldtype) {
    case "glpiselect":
    case "dropdown":
    case "itemtype": {
      if (value == null || value === "") return "";
      const id = extractID(value);
      return id != null ? id : "";
    }
    case "actor": {
      // Single → int, multi (array) → int[]
      if (Array.isArray(value)) {
        return value.map(extractID).filter((x): x is number => x != null);
      }
      if (value == null || value === "") return "";
      const id = extractID(value);
      return id != null ? id : "";
    }
    case "tag": {
      if (!Array.isArray(value)) return [];
      return value.map(extractID).filter((x): x is number => x != null);
    }
    case "file": {
      if (!Array.isArray(value)) return [];
      return value
        .map((doc) => {
          const obj = doc as { id?: unknown };
          const id = typeof obj?.id === "number" ? obj.id : parseInt(String(obj?.id ?? ""), 10);
          return Number.isFinite(id) ? id : null;
        })
        .filter((x): x is number => x != null);
    }
    default:
      return value;
  }
}

export default function FormcreatorFormDrawer({ open, onClose, formID, profileID }: Props) {
  const queryClient = useQueryClient();
  const [answers, setAnswers] = useState<Record<string, unknown>>({});
  const [submitResult, setSubmitResult] = useState<{
    form_answer_id: number;
    url?: string;
    created_tickets?: { id: number; url: string }[];
  } | null>(null);

  const { data: bundle, isLoading, error } = useQuery({
    queryKey: ["glpi-form", formID, profileID],
    queryFn: () => glpiAPI.getFormBundle(formID!, profileID!),
    enabled: open && !!formID && !!profileID,
    retry: false,
    staleTime: 5 * 60_000,
  });

  // Reset state every time the drawer opens with a new form (or closes).
  useEffect(() => {
    if (!open) return;
    setAnswers({});
    setSubmitResult(null);
  }, [open, formID, profileID]);

  // Seed default values once the bundle arrives.
  useEffect(() => {
    if (!bundle) return;
    setAnswers((prev) => {
      // Only seed empty form, never overwrite user input.
      if (Object.keys(prev).length > 0) return prev;
      const seed: Record<string, unknown> = {};
      for (const q of bundle.questions ?? []) {
        if (q.default_values) seed[String(q.id)] = q.default_values;
      }
      return seed;
    });
  }, [bundle]);

  // ── Index conditions and questions by section/question ───────────────────
  const { questionConds, sectionConds, sortedSections, questionsBySection } = useMemo(() => {
    const qConds = new Map<number, FormcreatorCondition[]>();
    const sConds = new Map<number, FormcreatorCondition[]>();
    for (const c of bundle?.conditions ?? []) {
      const map = c.itemtype === "PluginFormcreatorSection" ? sConds : qConds;
      const arr = map.get(c.items_id) ?? [];
      arr.push(c);
      map.set(c.items_id, arr);
    }
    const sections: FormcreatorSection[] = [...(bundle?.sections ?? [])].sort(
      (a, b) => a.order - b.order
    );
    const qbs = new Map<number, Question[]>();
    for (const q of bundle?.questions ?? []) {
      const arr = qbs.get(q.plugin_formcreator_sections_id) ?? [];
      arr.push(q);
      qbs.set(q.plugin_formcreator_sections_id, arr);
    }
    qbs.forEach((arr) => arr.sort((a, b) => a.order - b.order));
    return { questionConds: qConds, sectionConds: sConds, sortedSections: sections, questionsBySection: qbs };
  }, [bundle]);

  // ── Validation: required + visible + empty + supported ───────────────────
  const validationErrors = useMemo(() => {
    const errors: Record<string, string> = {};
    let blockingUnsupported = false;
    for (const q of bundle?.questions ?? []) {
      const visible = isVisible(questionConds.get(q.id) ?? [], answers);
      if (!visible) continue;
      const supported = SUPPORTED_QUESTION_TYPES.has(q.fieldtype) || q.fieldtype === "description";
      if (q.required === 1 && !supported) {
        blockingUnsupported = true;
      }
      if (
        q.required === 1 &&
        supported &&
        q.fieldtype !== "description" &&
        q.fieldtype !== "hidden"
      ) {
        if (isEmptyForType(q.fieldtype, answers[String(q.id)])) {
          errors[String(q.id)] = "Obrigatório";
        }
      }
    }
    return { errors, blockingUnsupported };
  }, [bundle, answers, questionConds]);

  const submitMutation = useMutation({
    mutationFn: () => {
      // Produce the wire-shape answers map: strip picker labels, reduce file
      // lists to doc-id arrays, pass primitives through unchanged.
      const qByID = new Map<string, Question>();
      for (const q of bundle?.questions ?? []) qByID.set(String(q.id), q);
      const wire: Record<string, unknown> = {};
      for (const [qid, val] of Object.entries(answers)) {
        const q = qByID.get(qid);
        wire[qid] = q ? normalizeAnswer(q.fieldtype, val) : val;
      }
      return glpiAPI.submitForm(formID!, profileID!, wire);
    },
    onSuccess: (res) => {
      setSubmitResult(res);
      // Refresh ticket lists so the spawned ticket appears immediately.
      queryClient.invalidateQueries({ queryKey: ["global-chamados"] });
      queryClient.invalidateQueries({ queryKey: ["profile-chamados"] });
    },
  });

  const handleSubmit = () => {
    if (Object.keys(validationErrors.errors).length > 0) return;
    if (validationErrors.blockingUnsupported) return;
    submitMutation.mutate();
  };

  const title = bundle?.form?.name ?? "Formulário";

  const footer = !submitResult && (
    <div className="flex items-center justify-between gap-3 px-4 py-3 border-t border-[var(--border-subtle)]">
      <span className="text-[11px] text-[var(--text-muted)]">
        {validationErrors.blockingUnsupported
          ? "Este formulário usa um tipo de campo obrigatório que não é suportado em sshcm. Abra no GLPI."
          : Object.keys(validationErrors.errors).length > 0
          ? "Preencha os campos obrigatórios destacados."
          : "Pronto para enviar."}
      </span>
      <div className="flex gap-2">
        <Button type="button" variant="secondary" onClick={onClose}>
          Cancelar
        </Button>
        <Button
          type="button"
          onClick={handleSubmit}
          loading={submitMutation.isPending}
          disabled={
            !!validationErrors.blockingUnsupported ||
            Object.keys(validationErrors.errors).length > 0
          }
        >
          Enviar
        </Button>
      </div>
    </div>
  );

  return (
    <Drawer open={open} onClose={onClose} title={title} wide footer={footer}>
      <div className="p-4 space-y-5">
        {isLoading && (
          <div className="space-y-3">
            <Skeleton className="h-5 w-1/2 rounded" />
            <Skeleton className="h-12 w-full rounded-[var(--radius-md)]" />
            <Skeleton className="h-12 w-full rounded-[var(--radius-md)]" />
            <Skeleton className="h-12 w-full rounded-[var(--radius-md)]" />
          </div>
        )}

        {error && (
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-3 py-2">
            Falha ao carregar o formulário: {(error as Error).message}
          </div>
        )}

        {bundle && submitMutation.isError && !submitResult && (
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-3 py-2">
            Falha ao enviar: {(submitMutation.error as Error).message}
          </div>
        )}

        {bundle?.warnings?.length ? (
          <div className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 text-amber-300 text-[11px] px-3 py-2 space-y-1">
            {bundle.warnings.map((w, i) => (
              <p key={i}>{w}</p>
            ))}
          </div>
        ) : null}

        {/* ── Success pane ──────────────────────────────────────────────── */}
        {submitResult && (
          <div className="space-y-4">
            <div className="rounded-[var(--radius-md)] border border-emerald-500/30 bg-emerald-500/10 text-emerald-300 text-sm px-4 py-3">
              <p className="font-semibold">Formulário enviado.</p>
              <p className="text-xs mt-1">
                ID da resposta: <code>#{submitResult.form_answer_id}</code>
              </p>
            </div>
            {submitResult.created_tickets?.length ? (
              <div className="space-y-1">
                <p className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider">
                  Chamados criados
                </p>
                {submitResult.created_tickets.map((t) => (
                  <Link
                    key={t.id}
                    href={t.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="block text-sm text-[var(--accent)] hover:underline"
                  >
                    #{t.id} ↗
                  </Link>
                ))}
              </div>
            ) : null}
            <div className="flex gap-2">
              {submitResult.url && (
                <Link
                  href={submitResult.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-1 px-3 py-1.5 rounded-[var(--radius-md)] bg-[var(--accent)] text-white text-sm font-medium hover:opacity-90"
                >
                  Abrir no GLPI
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
                  </svg>
                </Link>
              )}
              <Button type="button" variant="secondary" onClick={onClose}>
                Fechar
              </Button>
            </div>
          </div>
        )}

        {/* ── Form body ─────────────────────────────────────────────────── */}
        {bundle && !submitResult && (
          <div className="space-y-6">
            {bundle.form.description && (
              <p className="text-sm text-[var(--text-muted)]">{bundle.form.description}</p>
            )}
            {sortedSections.map((section) => {
              const sectionVisible = isVisible(sectionConds.get(section.id) ?? [], answers);
              if (!sectionVisible) return null;
              const qs = questionsBySection.get(section.id) ?? [];
              return (
                <section key={section.id}>
                  {section.name && (
                    <h3 className="text-xs font-semibold text-[var(--text-faint)] uppercase tracking-wider mb-3">
                      {section.name}
                    </h3>
                  )}
                  <div className="space-y-4">
                    {qs.map((q) => (
                      <FormcreatorQuestion
                        key={q.id}
                        question={q}
                        value={answers[String(q.id)]}
                        onChange={(v) =>
                          setAnswers((prev) => ({ ...prev, [String(q.id)]: v }))
                        }
                        visible={isVisible(questionConds.get(q.id) ?? [], answers)}
                        formID={bundle.form.id}
                        glpiBaseURL={bundle.glpi_base_url}
                        profileID={profileID}
                        error={validationErrors.errors[String(q.id)]}
                      />
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        )}
      </div>
    </Drawer>
  );
}
