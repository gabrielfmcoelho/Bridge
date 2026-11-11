"use client";

import { useEffect, useState } from "react";
import { useMutation } from "@tanstack/react-query";
import { glpiSeadAPI } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import Select from "@/components/ui/Select";
import RadioGroup from "@/components/ui/RadioGroup";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import { glpiAncestorPath } from "@/lib/glpiHierarchy";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated?: (ticketID: number) => void;
}

const PRIORITY_MATRIX: number[][] = [
  [1, 1, 2, 2, 3],
  [1, 2, 2, 3, 4],
  [2, 2, 3, 4, 4],
  [2, 3, 4, 4, 5],
  [3, 4, 4, 5, 5],
];
function computePriority(u: number, i: number): number {
  return PRIORITY_MATRIX[Math.min(Math.max(u, 1), 5) - 1]?.[Math.min(Math.max(i, 1), 5) - 1] ?? 3;
}

const URGENCY_VALUES = ["1", "2", "3", "4", "5"];
const URGENCY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.urgencyVeryLow",
  "2": "glpi.urgencyLow",
  "3": "glpi.urgencyMedium",
  "4": "glpi.urgencyHigh",
  "5": "glpi.urgencyVeryHigh",
};
const IMPACT_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.impactVeryLow",
  "2": "glpi.impactLow",
  "3": "glpi.impactMedium",
  "4": "glpi.impactHigh",
  "5": "glpi.impactVeryHigh",
};
const PRIORITY_VALUES = ["1", "2", "3", "4", "5"];
const PRIORITY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.priorityVeryLow",
  "2": "glpi.priorityLow",
  "3": "glpi.priorityMedium",
  "4": "glpi.priorityHigh",
  "5": "glpi.priorityVeryHigh",
};

function itemFromPicker(item: AsyncPickerItem | null): { id: number | null; label?: string } {
  return { id: item?.id ?? null, label: item?.label };
}

export default function CreateSeadTicketModal({ open, onClose, onCreated }: Props) {
  const { t } = useLocale();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [type, setType] = useState("1");
  const [category, setCategory] = useState<{ id: number | null; label?: string }>({ id: null });
  const [location, setLocation] = useState<{ id: number | null; label?: string }>({ id: null });
  const [requestType, setRequestType] = useState<{ id: number | null; label?: string }>({ id: null });
  const [urgency, setUrgency] = useState("3");
  const [impact, setImpact] = useState("3");
  const [priority, setPriority] = useState("3");
  const [priorityTouched, setPriorityTouched] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (priorityTouched) return;
    setPriority(String(computePriority(parseInt(urgency, 10), parseInt(impact, 10))));
  }, [urgency, impact, priorityTouched]);

  useEffect(() => {
    if (open) {
      setTitle("");
      setDescription("");
      setType("1");
      setCategory({ id: null });
      setLocation({ id: null });
      setRequestType({ id: null });
      setUrgency("3");
      setImpact("3");
      setPriority("3");
      setPriorityTouched(false);
      setError(null);
    }
  }, [open]);

  const dropdownFetcher = (itemtype: string) => async (q: string) => {
    const res = await glpiSeadAPI.searchDropdown(itemtype, q);
    return res.items.map(
      (r) =>
        ({
          id: r.id,
          label: r.name || `#${r.id}`,
          path: glpiAncestorPath(r.completename, r.name),
        } as AsyncPickerItem)
    );
  };

  const mutation = useMutation({
    mutationFn: () =>
      glpiSeadAPI.createTicket({
        title: title.trim(),
        description: description.trim() || undefined,
        type: parseInt(type, 10),
        urgency: parseInt(urgency, 10),
        impact: parseInt(impact, 10),
        priority: parseInt(priority, 10),
        category_id: category.id || undefined,
        location_id: location.id || undefined,
        request_type_id: requestType.id || undefined,
      }),
    onSuccess: (res) => {
      onCreated?.(res.ticket_id);
      onClose();
    },
    onError: (err: Error) => setError(err.message),
  });

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={t("glpiSead.createTicketTitle")}
      size="lg"
      footer={
        <FormFooter
          onCancel={onClose}
          cancelLabel={t("common.cancel")}
          submitLabel={t("glpi.createTicketSubmit")}
          onSubmit={() => mutation.mutate()}
          loading={mutation.isPending}
          disabled={!title.trim()}
        />
      }
    >
      <div className="space-y-5">
        <RadioGroup
          label={t("glpi.typeLabel")}
          options={[
            { value: "1", label: t("glpi.typeIncident") },
            { value: "2", label: t("glpi.typeRequest") },
          ]}
          value={type}
          onChange={setType}
        />
        <Input
          label={t("common.title")}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("glpi.shortSummaryPlaceholder")}
        />
        <div>
          <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">{t("common.description")}</label>
          <MarkdownEditor value={description} onChange={setDescription} rows={6} placeholder={t("glpi.describeIssuePlaceholder")} />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <AsyncPicker
            label={t("glpi.categoryLabel")}
            value={category.id}
            selectedLabel={category.label}
            fetcher={dropdownFetcher("ITILCategory")}
            onChange={(item) => setCategory(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "ITILCategory" })}
          />
          <AsyncPicker
            label={t("glpi.requestTypeLabel")}
            value={requestType.id}
            selectedLabel={requestType.label}
            fetcher={dropdownFetcher("RequestType")}
            onChange={(item) => setRequestType(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "RequestType" })}
          />
          <AsyncPicker
            label={t("glpi.locationLabel")}
            value={location.id}
            selectedLabel={location.label}
            fetcher={dropdownFetcher("Location")}
            onChange={(item) => setLocation(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "Location" })}
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <Select
            label={t("glpi.urgencyLabel")}
            options={URGENCY_VALUES.map((v) => ({ value: v, label: t(URGENCY_LABEL_KEYS[v]) }))}
            value={urgency}
            onChange={(e) => setUrgency(e.target.value)}
          />
          <Select
            label={t("glpi.impactLabel")}
            options={URGENCY_VALUES.map((v) => ({ value: v, label: t(IMPACT_LABEL_KEYS[v]) }))}
            value={impact}
            onChange={(e) => setImpact(e.target.value)}
          />
          <Select
            label={t("glpi.priorityLabel")}
            options={PRIORITY_VALUES.map((v) => ({ value: v, label: t(PRIORITY_LABEL_KEYS[v]) }))}
            value={priority}
            onChange={(e) => {
              setPriorityTouched(true);
              setPriority(e.target.value);
            }}
          />
        </div>

        {error && (
          <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-xs px-3 py-2">
            {error}
          </div>
        )}
      </div>
    </ResponsiveModal>
  );
}
