"use client";

import { useEffect, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import { Skeleton } from "@/components/ui/Skeleton";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import DynamicField from "@/components/requests/DynamicField";
import { offeringsAPI, requestsAPI, enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { CatalogHit, FormField, Offering } from "@/lib/types";

// Defaults each schema field to a type-appropriate empty value so form_data
// always carries every field's key, not just the ones the user touched.
function emptyValues(fields: FormField[]): Record<string, unknown> {
  const values: Record<string, unknown> = {};
  for (const f of fields) {
    values[f.key] = f.type === "checkbox" ? false : f.type === "tags" ? [] : f.type === "asset_ref" ? null : "";
  }
  return values;
}

interface RequestFormModalProps {
  /** The offering the user wants to request, as picked from catalog search.
   *  null closes the modal. CatalogHit does not carry form_schema, so the
   *  full Offering is fetched by id below once this is non-null. */
  hit: CatalogHit | null;
  onClose: () => void;
}

export default function RequestFormModal({ hit, onClose }: RequestFormModalProps) {
  const { t } = useLocale();
  const router = useRouter();
  const queryClient = useQueryClient();

  const [title, setTitle] = useState("");
  const [priority, setPriority] = useState("");
  const [values, setValues] = useState<Record<string, unknown>>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submitError, setSubmitError] = useState("");

  const open = hit != null;

  const offeringQuery = useQuery({
    queryKey: ["offering", hit?.id],
    queryFn: () => offeringsAPI.get(hit!.id),
    enabled: open,
  });
  const offering: Offering | undefined = offeringQuery.data;

  const { data: priorityEnum = [] } = useQuery({
    queryKey: ["enums", "issue_priority"],
    queryFn: () => enumsAPI.list("issue_priority"),
    enabled: open,
  });

  // Reset the form every time a new offering is opened, and seed form_data
  // defaults once its schema arrives (fetched separately — see the module
  // doc comment on RequestFormModalProps.hit). Both are external→state syncs
  // the set-state-in-effect rule flags; EditApiModal.tsx establishes the same
  // disable-comment pattern for the same reason.
  /* eslint-disable react-hooks/set-state-in-effect -- resetting local form
     state when a (possibly different) offering is opened / its schema loads. */
  useEffect(() => {
    if (open) {
      setTitle("");
      setPriority("");
      setErrors({});
      setSubmitError("");
    }
  }, [open, hit?.id]);

  useEffect(() => {
    if (offering) setValues(emptyValues(offering.form_schema.fields));
  }, [offering]);
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

  const validate = (o: Offering): Record<string, string> => {
    const errs: Record<string, string> = {};
    if (!title.trim()) errs.title = t("requests.form.required");

    for (const field of o.form_schema.fields) {
      const v = values[field.key];
      const empty =
        field.type === "tags" ? !Array.isArray(v) || v.length === 0 :
        field.type === "checkbox" ? false :
        v === undefined || v === null || v === "";

      if (field.required && empty) {
        errs[field.key] = t("requests.form.required");
        continue;
      }
      if (empty) continue;

      if (field.type === "number" && (typeof v !== "number" || Number.isNaN(v))) {
        errs[field.key] = t("requests.form.invalidNumber");
      } else if ((field.type === "text" || field.type === "textarea") && field.max_length && typeof v === "string" && v.length > field.max_length) {
        errs[field.key] = t("requests.form.maxLength", { max: String(field.max_length) });
      } else if (field.type === "select" && field.options && !field.options.includes(String(v))) {
        errs[field.key] = t("requests.form.invalidOption");
      }
    }
    return errs;
  };

  const mutation = useMutation({
    mutationFn: () =>
      requestsAPI.create({
        offering_id: offering!.id,
        title: title.trim(),
        priority: priority || undefined,
        form_data: values,
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
    const errs = validate(offering);
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
      title={t("requests.new")}
      subHeader={hit && <p className="text-sm text-[var(--text-secondary)] truncate">{hit.name}</p>}
      footer={
        <div className="flex gap-2 justify-end">
          <Button type="button" variant="secondary" onClick={onClose} disabled={mutation.isPending}>
            {t("common.cancel")}
          </Button>
          <Button type="button" onClick={handleSubmit} loading={mutation.isPending} disabled={!offering || mutation.isPending}>
            {t("catalog.request")}
          </Button>
        </div>
      }
    >
      {offeringQuery.isLoading ? (
        <div className="space-y-3">
          <Skeleton className="h-9 w-full rounded-[var(--radius-md)]" />
          <Skeleton className="h-9 w-full rounded-[var(--radius-md)]" />
          <Skeleton className="h-9 w-full rounded-[var(--radius-md)]" />
          <Skeleton className="h-24 w-full rounded-[var(--radius-md)]" />
        </div>
      ) : offeringQuery.isError || !offering ? (
        <StatusAlert variant="error">{t("requests.form.loadError")}</StatusAlert>
      ) : (
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

          {offering.form_schema.fields.map((field) => (
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
    </ResponsiveModal>
  );
}
