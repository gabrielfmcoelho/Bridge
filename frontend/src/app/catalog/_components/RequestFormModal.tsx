"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Badge from "@/components/ui/Badge";
import Icon from "@/components/ui/Icon";
import StepIndicator from "@/components/ui/StepIndicator";
import { Skeleton } from "@/components/ui/Skeleton";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import DynamicField from "@/components/requests/DynamicField";
import RequestAnswers from "@/components/requests/RequestAnswers";
import { offeringsAPI, requestsAPI, enumsAPI } from "@/lib/api";
import { ICON_PATHS, REQUEST_TYPE_ICON } from "@/lib/icon-paths";
import { emptyFormValues, validateFormData, visibleFields, visibleValues } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import type { Offering } from "@/lib/types";
import AiPrefillPanel, { type AiDraft } from "./AiPrefillPanel";
import TemplatePicker from "./TemplatePicker";

interface RequestFormModalProps {
  /** The offering the user picked, or null when closed. Only id and name are
   *  needed here — the full Offering (with its form_schema) is fetched by id
   *  below, so callers never have to carry a schema around to open this. */
  target: { id: number; name: string } | null;
  onClose: () => void;
}

/** Steps present for every offering; "start" is inserted only when the offering
 *  actually declares templates, so nobody clicks through an empty screen. */
type StepId = "requester" | "start" | "details" | "review";

function StepHeading({ title, hint, icon }: { title: string; hint?: string; icon: string }) {
  return (
    <div className="space-y-1 border-b border-[var(--border-subtle)] pb-2">
      <div className="flex items-center gap-2">
        <Icon path={icon} className="h-3.5 w-3.5 text-[var(--text-faint)]" strokeWidth={2} />
        <h3 className="text-xs font-semibold uppercase tracking-wider text-[var(--text-secondary)]">{title}</h3>
      </div>
      {hint && <p className="text-xs leading-relaxed text-[var(--text-muted)]">{hint}</p>}
    </div>
  );
}

function ReviewRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-1 sm:grid-cols-3 sm:gap-3">
      <dt className="text-xs font-medium tracking-wide text-[var(--text-muted)] sm:pt-0.5">{label}</dt>
      <dd className="text-sm text-[var(--text-primary)] sm:col-span-2">{children}</dd>
    </div>
  );
}

export default function RequestFormModal({ target, onClose }: RequestFormModalProps) {
  const { t } = useLocale();
  const { user } = useAuth();
  const router = useRouter();
  const queryClient = useQueryClient();

  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState("");
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");
  const [templateKey, setTemplateKey] = useState<string | null>(null);
  const [entidadeId, setEntidadeId] = useState<string>("");
  const [contactName, setContactName] = useState("");
  const [contactPhone, setContactPhone] = useState("");
  const [stepIndex, setStepIndex] = useState(0);

  const open = target != null;

  const offeringQuery = useQuery({
    queryKey: ["offering", target?.id],
    queryFn: () => offeringsAPI.get(target!.id),
    enabled: open,
  });
  const offering: Offering | undefined = offeringQuery.data;

  const { data: priorityEnum = [] } = useQuery({
    queryKey: ["enums", "issue_priority"],
    queryFn: () => enumsAPI.list("issue_priority"),
    enabled: open,
  });

  const entidades = user?.entidades ?? [];
  // Depend on the id, not the object: the array is rebuilt every render, so an
  // object dep would reset the form on every keystroke.
  const primaryEntidadeId = (entidades.find((e) => e.is_primary) ?? entidades[0])?.id;

  /* eslint-disable react-hooks/set-state-in-effect -- resetting local form
     state when a (possibly different) offering is opened / its schema loads. */
  useEffect(() => {
    if (open) {
      setTitle("");
      setPriority("");
      setErrors({});
      setSubmitError("");
      setTemplateKey(null);
      setStepIndex(0);
      setEntidadeId(primaryEntidadeId != null ? String(primaryEntidadeId) : "");
      setContactName(user?.display_name ?? "");
      setContactPhone("");
    }
  }, [open, target?.id, primaryEntidadeId, user?.display_name]);

  useEffect(() => {
    if (offering) setValues(emptyFormValues(offering.form_schema.fields));
  }, [offering]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const hasTemplates = (offering?.templates?.length ?? 0) > 0;
  const steps: StepId[] = ["requester", ...(hasTemplates ? (["start"] as StepId[]) : []), "details", "review"];
  const step = steps[Math.min(stepIndex, steps.length - 1)];
  const isFirst = stepIndex === 0;
  const isLast = stepIndex === steps.length - 1;

  const setFieldValue = (key: string, value: unknown) => {
    setValues((v) => ({ ...v, [key]: value }));
    setErrors((e) => {
      if (!e[key]) return e;
      const next = { ...e };
      delete next[key];
      return next;
    });
  };

  // A template writes the same state typing would, so every value it sets stays
  // editable and is validated by the same rules. Choosing "custom" resets to the
  // schema's blanks rather than leaving the last preset's numbers behind.
  const applyTemplate = (key: string | null) => {
    if (!offering) return;
    setTemplateKey(key);
    const blanks = emptyFormValues(offering.form_schema.fields);
    const tpl = key ? offering.templates?.find((x) => x.key === key) : undefined;
    setValues(tpl ? { ...blanks, ...tpl.values } : blanks);
    setErrors({});
  };

  // An AI draft is applied exactly like typing would be: same state, same rules,
  // every field still editable. Nothing is submitted on the user's behalf.
  const applyDraft = (draft: AiDraft) => {
    if (draft.title) setTitle(draft.title);
    if (draft.priority) setPriority(draft.priority);
    setValues((v) => ({ ...v, ...draft.form_data }));
    setErrors({});
  };

  const validateDetails = (o: Offering): Record<string, string> => {
    const errs = validateFormData(o.form_schema.fields, values, t);
    // Title is not part of form_schema — it is the request's own field.
    if (!title.trim()) errs.title = t("requests.form.required");
    return errs;
  };

  // Advancing off "details" is the gate: you cannot reach the review of a form
  // that would be rejected, so the last step always shows a sendable request.
  const goNext = () => {
    if (!offering) return;
    if (step === "requester") {
      if (!contactName.trim()) {
        setErrors({ contact_name: t("requests.form.required") });
        return;
      }
      setErrors({});
    }
    if (step === "details") {
      const errs = validateDetails(offering);
      setErrors(errs);
      if (Object.keys(errs).length > 0) return;
    }
    setStepIndex((i) => Math.min(i + 1, steps.length - 1));
  };

  const goBack = () => {
    setSubmitError("");
    setStepIndex((i) => Math.max(i - 1, 0));
  };

  const mutation = useMutation({
    mutationFn: () =>
      requestsAPI.create({
        offering_id: offering!.id,
        title: title.trim(),
        priority: priority || undefined,
        requester_entidade_id: entidadeId ? Number(entidadeId) : undefined,
        contact_name: contactName.trim(),
        contact_phone: contactPhone.trim() || undefined,
        // Only what the user could see — a field hidden by depends_on must not
        // ship an answer nobody gave.
        form_data: visibleValues(offering!.form_schema.fields, values),
      }),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: ["requests"] });
      onClose();
      router.push(`/requests/${created.id}`);
    },
    onError: (err) => setSubmitError(err instanceof Error ? err.message : t("requests.form.submitError")),
  });

  const handleSubmit = () => {
    if (!offering) return;
    setSubmitError("");
    const errs = validateDetails(offering);
    if (Object.keys(errs).length > 0) {
      // Should be unreachable — the details gate runs first — but a schema whose
      // depends_on changed under the user must not submit silently.
      setErrors(errs);
      setStepIndex(steps.indexOf("details"));
      return;
    }
    mutation.mutate();
  };

  const priorityOptions = priorityEnum.map((o) => ({ value: o.value, label: t(`requests.priorityLevels.${o.value}`) }));
  const entidadeName = entidades.find((e) => String(e.id) === entidadeId)?.name;
  const shownFields = offering ? visibleFields(offering.form_schema.fields, values) : [];

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={t("requests.new")}
      subHeader={
        offering ? (
          <div className="space-y-3">
            <div className="flex items-center gap-2.5">
              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--accent)]">
                <Icon path={REQUEST_TYPE_ICON[offering.request_type]} className="h-4 w-4" strokeWidth={1.5} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-[var(--text-primary)]">{offering.name}</span>
                <span className="block truncate text-xs text-[var(--text-muted)]">{offering.category}</span>
              </span>
              <Badge color="gray">{t(`catalog.requestType.${offering.request_type}`)}</Badge>
            </div>
            {/* Progress lives in the pinned header, so "where am I" survives scrolling. */}
            <StepIndicator steps={steps.map((s) => t(`catalog.steps.${s}`))} current={stepIndex + 1} />
          </div>
        ) : (
          target && <p className="truncate text-sm text-[var(--text-secondary)]">{target.name}</p>
        )
      }
      footer={
        <div className={`grid gap-3 ${isFirst ? "grid-cols-2" : "grid-cols-3"}`}>
          <Button type="button" variant="secondary" className="w-full" onClick={onClose} disabled={mutation.isPending}>
            <Icon path={ICON_PATHS.close} className="h-4 w-4" strokeWidth={2} />
            {t("common.cancel")}
          </Button>

          {/* Back sits between cancel and the forward action, so the destructive
              exit and the reversible one are never adjacent. */}
          {!isFirst && (
            <Button type="button" variant="secondary" className="w-full" onClick={goBack} disabled={mutation.isPending}>
              <Icon path={ICON_PATHS.back} className="h-4 w-4" strokeWidth={2} />
              {t("catalog.steps.back")}
            </Button>
          )}

          {isLast ? (
            <Button type="button" className="w-full" onClick={handleSubmit} loading={mutation.isPending} disabled={!offering || mutation.isPending}>
              {!mutation.isPending && <Icon path={ICON_PATHS.checkCircle} className="h-4 w-4" strokeWidth={2} />}
              {t("catalog.request")}
            </Button>
          ) : (
            <Button type="button" className="w-full" onClick={goNext} disabled={!offering}>
              {t("catalog.steps.next")}
              <Icon path={ICON_PATHS.chevronUp} className="h-4 w-4 rotate-90" strokeWidth={2} />
            </Button>
          )}
        </div>
      }
    >
      {offeringQuery.isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-9 w-full rounded-[var(--radius-md)]" />
          <Skeleton className="h-9 w-full rounded-[var(--radius-md)]" />
          <Skeleton className="h-24 w-full rounded-[var(--radius-md)]" />
        </div>
      ) : offeringQuery.isError || !offering ? (
        <StatusAlert variant="error">{t("requests.form.loadError")}</StatusAlert>
      ) : (
        <div className="space-y-5">
          {submitError && <StatusAlert variant="error">{submitError}</StatusAlert>}

          {step === "requester" && (
            <div className="space-y-4">
              <StepHeading title={t("catalog.steps.requester")} icon={ICON_PATHS.user} hint={t("catalog.steps.contactHint")} />

              <div className="space-y-1.5">
                <label className="block text-xs font-medium tracking-wide text-[var(--text-secondary)]">
                  {t("catalog.steps.contactName")}
                  <span className="ml-0.5 text-[var(--danger)]">*</span>
                </label>
                <Input
                  value={contactName}
                  onChange={(e) => {
                    setContactName(e.target.value);
                    if (errors.contact_name) setErrors((prev) => { const next = { ...prev }; delete next.contact_name; return next; });
                  }}
                />
                {errors.contact_name && <FormError message={errors.contact_name} />}
              </div>

              <div className="space-y-1.5">
                <label className="block text-xs font-medium tracking-wide text-[var(--text-secondary)]">
                  {t("catalog.steps.contactPhone")}
                </label>
                {/* type=tel gets the numeric keypad on mobile without rejecting
                    the parentheses, spaces and hyphens people actually type. */}
                <Input
                  type="tel"
                  inputMode="tel"
                  value={contactPhone}
                  placeholder={t("catalog.steps.phonePlaceholder")}
                  onChange={(e) => setContactPhone(e.target.value)}
                />
                <p className="text-xs text-[var(--text-muted)]">{t("catalog.steps.phoneOptional")}</p>
              </div>

              {entidades.length > 1 ? (
                <>
                  <Select
                    label={t("catalog.steps.onBehalf")}
                    value={entidadeId}
                    onChange={(e) => setEntidadeId(e.target.value)}
                    options={entidades.map((e) => ({ value: String(e.id), label: e.name }))}
                  />
                  <p className="text-xs text-[var(--text-muted)]">{t("catalog.steps.multiEntidadeHint")}</p>
                </>
              ) : entidades.length === 1 ? (
                <>
                  <ReviewRow label={t("catalog.steps.onBehalf")}>{entidades[0].name}</ReviewRow>
                  <p className="text-xs text-[var(--text-faint)]">{t("catalog.steps.singleEntidade")}</p>
                </>
              ) : (
                <StatusAlert variant="warning">{t("catalog.steps.noEntidade")}</StatusAlert>
              )}
            </div>
          )}

          {step === "start" && offering.templates && (
            <div className="space-y-4">
              <StepHeading title={t("catalog.templates.title")} icon={ICON_PATHS.cube} />
              <TemplatePicker
                templates={offering.templates}
                fields={offering.form_schema.fields}
                selected={templateKey}
                onSelect={applyTemplate}
              />
            </div>
          )}

          {step === "details" && (
            <div className="space-y-5">
              <StepHeading title={t("requests.form.sectionDetails")} icon={ICON_PATHS.gear} />
              <AiPrefillPanel offeringId={offering.id} onDraft={applyDraft} />

              <div className="space-y-1.5">
                <label className="block text-xs font-medium tracking-wide text-[var(--text-secondary)]">
                  {t("requests.titleField")}
                  <span className="ml-0.5 text-[var(--danger)]">*</span>
                </label>
                <Input
                  value={title}
                  placeholder={t("requests.form.titlePlaceholder")}
                  onChange={(e) => {
                    setTitle(e.target.value);
                    if (errors.title) setErrors((prev) => { const next = { ...prev }; delete next.title; return next; });
                  }}
                />
                {errors.title && <FormError message={errors.title} />}
              </div>

              <Select label={t("requests.priority")} value={priority} onChange={(e) => setPriority(e.target.value)} options={priorityOptions} />

              {shownFields.map((field) => (
                <DynamicField
                  key={field.key}
                  field={field}
                  value={values[field.key]}
                  onChange={(v) => setFieldValue(field.key, v)}
                  error={errors[field.key]}
                />
              ))}
            </div>
          )}

          {step === "review" && (
            <div className="space-y-4">
              <StepHeading title={t("catalog.steps.review")} icon={ICON_PATHS.checkCircle} hint={t("catalog.steps.reviewIntro")} />
              <dl className="space-y-3">
                <ReviewRow label={t("requests.offering")}>{offering.name}</ReviewRow>
                <ReviewRow label={t("catalog.steps.contactName")}>{contactName.trim() || "—"}</ReviewRow>
                <ReviewRow label={t("catalog.steps.contactPhone")}>{contactPhone.trim() || "—"}</ReviewRow>
                <ReviewRow label={t("catalog.steps.onBehalf")}>{entidadeName ?? "—"}</ReviewRow>
                <ReviewRow label={t("requests.titleField")}>{title.trim() || "—"}</ReviewRow>
                <ReviewRow label={t("requests.priority")}>
                  {priority ? t(`requests.priorityLevels.${priority}`) : "—"}
                </ReviewRow>
              </dl>
              <div className="border-t border-[var(--border-subtle)] pt-4">
                {/* Rendered from the same schema+values that will be sent, so the
                    review cannot drift from the payload. */}
                <RequestAnswers schema={{ fields: shownFields }} data={values} />
              </div>
            </div>
          )}
        </div>
      )}
    </ResponsiveModal>
  );
}
