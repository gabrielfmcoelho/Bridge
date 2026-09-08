"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import DynamicField from "@/components/requests/DynamicField";
import { requestsAPI, enumsAPI } from "@/lib/api";
import { emptyFormValues, validateFormData, visibleFields, visibleValues } from "@/lib/requests";
import { useLocale } from "@/contexts/LocaleContext";
import type { ServiceRequest } from "@/lib/types";

interface RequestEditModalProps {
  /** The request to edit, or null when closed. */
  request: ServiceRequest | null;
  onClose: () => void;
  onDone: () => void;
}

// The requester's own edit, available only while `submitted`/`needs_info`
// (the server decides that — this modal is opened from can.edit).
//
// Renders form_schema_snapshot, NOT the offering's current form_schema: the
// snapshot is the schema this request was actually submitted against, and
// editing an old request must not silently re-shape its answers to whatever
// an admin changed the offering into since.
export default function RequestEditModal({ request, onClose, onDone }: RequestEditModalProps) {
  const { t } = useLocale();
  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState("");
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");

  const open = request != null;
  const fields = request?.form_schema_snapshot?.fields ?? [];

  const { data: priorityEnum = [] } = useQuery({
    queryKey: ["enums", "issue_priority"],
    queryFn: () => enumsAPI.list("issue_priority"),
    enabled: open,
  });

  /* eslint-disable react-hooks/set-state-in-effect -- seeding local form state
     from the loaded request, same pattern as RequestFormModal.tsx. */
  useEffect(() => {
    if (!request) return;
    setTitle(request.title);
    setPriority(request.priority);
    // Defaults first so a snapshot field the stored form_data never got a
    // value for still renders a control instead of an uncontrolled input.
    setValues({ ...emptyFormValues(request.form_schema_snapshot?.fields ?? []), ...request.form_data });
    setErrors({});
    setSubmitError("");
  }, [request]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const setFieldValue = (key: string, value: unknown) => {
    setValues((v) => ({ ...v, [key]: value }));
    setErrors((e) => {
      if (!e[key]) return e;
      const next = { ...e };
      delete next[key];
      return next;
    });
  };

  const mutation = useMutation({
    mutationFn: () =>
      requestsAPI.patch(request!.id, { title: title.trim(), priority, form_data: visibleValues(fields, values) }),
    onSuccess: () => {
      onDone();
      onClose();
    },
    onError: (err) => setSubmitError(err instanceof Error ? err.message : t("requests.editError")),
  });

  const handleSubmit = () => {
    setSubmitError("");
    const errs = validateFormData(fields, values, t);
    if (!title.trim()) errs.title = t("requests.form.required");
    setErrors(errs);
    if (Object.keys(errs).length === 0) mutation.mutate();
  };

  const priorityOptions = priorityEnum.map((o) => ({
    value: o.value,
    label: t(`requests.priorityLevels.${o.value}`),
  }));

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={t("requests.editTitle")}
      footer={
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            {t("common.cancel")}
          </Button>
          <Button type="button" onClick={handleSubmit} loading={mutation.isPending} disabled={mutation.isPending}>
            {t("common.save")}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {submitError && <StatusAlert variant="error">{submitError}</StatusAlert>}

        <div className="space-y-1.5">
          <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
            {t("requests.titleField")}
            <span className="text-[var(--danger)] ml-0.5">*</span>
          </label>
          <Input
            value={title}
            onChange={(e) => {
              setTitle(e.target.value);
              if (errors.title) setErrors((prev) => { const next = { ...prev }; delete next.title; return next; });
            }}
          />
          {errors.title && <FormError message={errors.title} />}
        </div>

        <Select label={t("requests.priority")} value={priority} onChange={(e) => setPriority(e.target.value)} options={priorityOptions} />

        {visibleFields(fields, values).map((field) => (
          <DynamicField
            key={field.key}
            field={field}
            value={values[field.key]}
            onChange={(v) => setFieldValue(field.key, v)}
            error={errors[field.key]}
          />
        ))}
      </div>
    </ResponsiveModal>
  );
}
