"use client";

import { useState, useEffect } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { glpiAPI, type GlpiTokenProfile } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import Select from "@/components/ui/Select";
import RadioGroup from "@/components/ui/RadioGroup";
import SectionHeading from "@/components/ui/SectionHeading";
import AsyncPicker, { type AsyncPickerItem } from "@/components/ui/AsyncPicker";
import { glpiAncestorPath } from "@/lib/glpiHierarchy";
import { useLocale } from "@/contexts/LocaleContext";

interface Props {
  open: boolean;
  onClose: () => void;
  // Context-aware defaults; the modal composes them into the create request.
  defaultProfileID?: number | null;
  defaultTitle?: string;
  defaultDescription?: string;
  defaultEntityID?: number;
  hostSlug?: string;
  alertID?: number;
  onCreated?: (ticketID: number, ticketURL: string) => void;
}

// GLPI's default urgency×impact priority matrix (CommonITILObject). Only a
// starting suggestion — the operator can always override the Priority select
// afterwards, same as in GLPI's own ticket form.
const PRIORITY_MATRIX: number[][] = [
  [1, 1, 2, 2, 3],
  [1, 2, 2, 3, 4],
  [2, 2, 3, 4, 4],
  [2, 3, 4, 4, 5],
  [3, 4, 4, 5, 5],
];
function computePriority(urgency: number, impact: number): number {
  const u = Math.min(Math.max(urgency, 1), 5) - 1;
  const i = Math.min(Math.max(impact, 1), 5) - 1;
  return PRIORITY_MATRIX[u]?.[i] ?? 3;
}

function itemFromPicker(item: AsyncPickerItem | null): { id: number | null; label?: string } {
  return { id: item?.id ?? null, label: item?.label };
}

// Shared "open GLPI ticket" modal. Used by project, host, and alert contexts —
// each passes whatever defaults it has. Relies on the admin having at least one
// token profile configured (queried on mount) — otherwise we show a setup hint.
// Mirrors GLPI's own Ticket::showForm fields: type, category, urgency/impact/
// priority, location, entity, request source, requester/observer/assigned
// actors — all backed by the same dropdown/user search endpoints Formcreator
// pickers use.
export default function CreateTicketModal({
  open,
  onClose,
  defaultProfileID,
  defaultTitle,
  defaultDescription,
  defaultEntityID,
  hostSlug,
  alertID,
  onCreated,
}: Props) {
  const { t } = useLocale();
  const [profileID, setProfileID] = useState<number | null>(defaultProfileID ?? null);
  const [title, setTitle] = useState(defaultTitle ?? "");
  const [description, setDescription] = useState(defaultDescription ?? "");
  const [type, setType] = useState("1"); // 1=incident 2=request

  const [entity, setEntity] = useState<{ id: number | null; label?: string }>({
    id: defaultEntityID || null,
  });
  const [category, setCategory] = useState<{ id: number | null; label?: string }>({ id: null });
  const [location, setLocation] = useState<{ id: number | null; label?: string }>({ id: null });
  const [requestType, setRequestType] = useState<{ id: number | null; label?: string }>({ id: null });
  const [slaTTO, setSlaTTO] = useState<{ id: number | null; label?: string }>({ id: null });
  const [slaTTR, setSlaTTR] = useState<{ id: number | null; label?: string }>({ id: null });

  const [urgency, setUrgency] = useState("3");
  const [impact, setImpact] = useState("3");
  const [priority, setPriority] = useState("3");
  const [priorityTouched, setPriorityTouched] = useState(false);

  const [requesters, setRequesters] = useState<AsyncPickerItem[]>([]);
  const [watchers, setWatchers] = useState<AsyncPickerItem[]>([]);
  const [assignUser, setAssignUser] = useState<AsyncPickerItem | null>(null);
  const [assignGroup, setAssignGroup] = useState<AsyncPickerItem | null>(null);

  const [linkComputer, setLinkComputer] = useState<boolean>(!!hostSlug);
  const [error, setError] = useState<string | null>(null);

  // Recompute the suggested priority whenever urgency/impact change, unless
  // the operator already picked a priority by hand.
  useEffect(() => {
    if (priorityTouched) return;
    setPriority(String(computePriority(parseInt(urgency, 10), parseInt(impact, 10))));
  }, [urgency, impact, priorityTouched]);

  // Reset state when the modal is re-opened with fresh defaults.
  useEffect(() => {
    if (open) {
      setProfileID(defaultProfileID ?? null);
      setTitle(defaultTitle ?? "");
      setDescription(defaultDescription ?? "");
      setType("1");
      setEntity({ id: defaultEntityID || null });
      setCategory({ id: null });
      setLocation({ id: null });
      setRequestType({ id: null });
      setSlaTTO({ id: null });
      setSlaTTR({ id: null });
      setUrgency("3");
      setImpact("3");
      setPriority("3");
      setPriorityTouched(false);
      setRequesters([]);
      setWatchers([]);
      setAssignUser(null);
      setAssignGroup(null);
      setLinkComputer(!!hostSlug);
      setError(null);
    }
  }, [open, defaultProfileID, defaultTitle, defaultDescription, defaultEntityID, hostSlug]);

  const { data: profiles } = useQuery<GlpiTokenProfile[]>({
    queryKey: ["glpi-profiles"],
    queryFn: glpiAPI.listProfiles,
    enabled: open,
    retry: false,
  });

  const dropdownFetcher = (itemtype: string) => async (q: string) => {
    if (!profileID) return [];
    const res = await glpiAPI.searchDropdown(itemtype, profileID, q);
    return res.items.map(
      (r) =>
        ({
          id: r.id,
          label: r.name,
          path: glpiAncestorPath(r.completename, r.name),
          disabled: r.disabled,
        } as AsyncPickerItem)
    );
  };
  const userFetcher = async (q: string) => {
    if (!profileID) return [];
    const res = await glpiAPI.searchUsers(profileID, q);
    return res.users.map(
      (u) =>
        ({
          id: u.id,
          label: u.display || u.login,
          secondary: u.login && u.display && u.login !== u.display ? u.login : undefined,
        } as AsyncPickerItem)
    );
  };

  const mutation = useMutation({
    mutationFn: () =>
      glpiAPI.createTicket({
        profile_id: profileID!,
        title: title.trim(),
        description: description.trim() || undefined,
        type: parseInt(type, 10),
        entity_id: entity.id || undefined,
        category_id: category.id || undefined,
        location_id: location.id || undefined,
        request_type_id: requestType.id || undefined,
        sla_id_tto: slaTTO.id || undefined,
        sla_id_ttr: slaTTR.id || undefined,
        urgency: parseInt(urgency, 10),
        impact: parseInt(impact, 10),
        priority: parseInt(priority, 10),
        requester_ids: requesters.length ? requesters.map((r) => r.id) : undefined,
        watcher_ids: watchers.length ? watchers.map((w) => w.id) : undefined,
        assign_user_id: assignUser?.id || undefined,
        assign_group_id: assignGroup?.id || undefined,
        host_slug: hostSlug,
        alert_id: alertID,
        link_computer: linkComputer,
      }),
    onSuccess: (res) => {
      if (res.warning) setError(res.warning); else setError(null);
      onCreated?.(res.ticket_id, res.ticket_url);
      if (!res.warning) {
        onClose();
      }
    },
    onError: (err: Error) => setError(err.message),
  });

  const hasProfiles = (profiles?.length ?? 0) > 0;

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={t("glpi.createTicketTitle")}
      size="lg"
      footer={
        <FormFooter
          onCancel={onClose}
          cancelLabel={t("common.cancel")}
          submitLabel={t("glpi.createTicketSubmit")}
          onSubmit={() => mutation.mutate()}
          loading={mutation.isPending}
          disabled={!hasProfiles || !profileID || !title.trim()}
        />
      }
    >
      <div className="space-y-5">
        {!hasProfiles && (
          <div className="rounded-[var(--radius-md)] border border-[var(--warning)]/30 bg-[var(--warning)]/10 text-[var(--warning)] text-xs px-3 py-2">
            {t("glpi.noProfilesConfigured")}
          </div>
        )}
        <div>
          <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">{t("glpi.profileLabel")}</label>
          <select
            value={profileID ?? ""}
            onChange={(e) => setProfileID(e.target.value ? parseInt(e.target.value, 10) : null)}
            disabled={!hasProfiles}
            className="w-full bg-[var(--bg-elevated)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-sm text-[var(--text-primary)] focus:outline-none focus:border-[var(--accent)]"
          >
            <option value="">{t("glpi.selectProfilePlaceholder")}</option>
            {(profiles ?? []).map((p) => (
              <option key={p.id} value={p.id}>{p.name}{p.description ? ` — ${p.description}` : ""}</option>
            ))}
          </select>
        </div>

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
          <MarkdownEditor
            value={description}
            onChange={setDescription}
            rows={6}
            placeholder={t("glpi.describeIssuePlaceholder")}
          />
        </div>

        <SectionHeading as="h3">{t("glpi.sectionClassification")}</SectionHeading>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <AsyncPicker
            label={t("glpi.categoryLabel")}
            value={category.id}
            selectedLabel={category.label}
            fetcher={dropdownFetcher("ITILCategory")}
            onChange={(item) => setCategory(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "ITILCategory" })}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.requestTypeLabel")}
            value={requestType.id}
            selectedLabel={requestType.label}
            fetcher={dropdownFetcher("RequestType")}
            onChange={(item) => setRequestType(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "RequestType" })}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.entityLabel")}
            value={entity.id}
            selectedLabel={entity.label}
            fetcher={dropdownFetcher("Entity")}
            onChange={(item) => setEntity(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "Entity" })}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.locationLabel")}
            value={location.id}
            selectedLabel={location.label}
            fetcher={dropdownFetcher("Location")}
            onChange={(item) => setLocation(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "Location" })}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.slaTTOLabel")}
            value={slaTTO.id}
            selectedLabel={slaTTO.label}
            fetcher={dropdownFetcher("SLA")}
            onChange={(item) => setSlaTTO(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "SLA" })}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.slaTTRLabel")}
            value={slaTTR.id}
            selectedLabel={slaTTR.label}
            fetcher={dropdownFetcher("SLA")}
            onChange={(item) => setSlaTTR(itemFromPicker(item))}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "SLA" })}
            disabled={!profileID}
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
            options={IMPACT_VALUES.map((v) => ({ value: v, label: t(IMPACT_LABEL_KEYS[v]) }))}
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

        <SectionHeading as="h3">{t("glpi.sectionPeople")}</SectionHeading>
        <AsyncPicker
          multi
          label={t("glpi.requesterLabel")}
          selectedItems={requesters}
          fetcher={userFetcher}
          onChange={setRequesters}
          placeholder={t("glpi.searchUsersPlaceholder")}
          disabled={!profileID}
        />
        <AsyncPicker
          multi
          label={t("glpi.watcherLabel")}
          selectedItems={watchers}
          fetcher={userFetcher}
          onChange={setWatchers}
          placeholder={t("glpi.searchUsersPlaceholder")}
          disabled={!profileID}
        />
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <AsyncPicker
            label={t("glpi.assignToUserLabel")}
            value={assignUser?.id ?? null}
            selectedLabel={assignUser?.label}
            fetcher={userFetcher}
            onChange={setAssignUser}
            placeholder={t("glpi.searchUserPlaceholder")}
            disabled={!profileID}
          />
          <AsyncPicker
            label={t("glpi.assignToGroupLabel")}
            value={assignGroup?.id ?? null}
            selectedLabel={assignGroup?.label}
            fetcher={dropdownFetcher("Group")}
            onChange={setAssignGroup}
            placeholder={t("glpi.searchItemtypePlaceholder", { itemtype: "Group" })}
            disabled={!profileID}
          />
        </div>

        {hostSlug && (
          <label className="flex items-center gap-2 text-xs text-[var(--text-secondary)] cursor-pointer">
            <input
              type="checkbox"
              checked={linkComputer}
              onChange={(e) => setLinkComputer(e.target.checked)}
              className="w-4 h-4 accent-[var(--accent)]"
            />
            {t("glpi.linkComputerCheckbox", { host: hostSlug })}
          </label>
        )}
        {error && (
          <div className="rounded-[var(--radius-md)] border border-[var(--danger)]/30 bg-[var(--danger)]/10 text-[var(--danger)] text-xs px-3 py-2">
            {error}
          </div>
        )}
      </div>
    </ResponsiveModal>
  );
}

// Rendered via t() at usage time — module scope can't call the hook. Mirrors
// FormcreatorQuestion's URGENCY_LABEL_KEYS convention.
const URGENCY_VALUES = ["1", "2", "3", "4", "5"];
const URGENCY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.urgencyVeryLow",
  "2": "glpi.urgencyLow",
  "3": "glpi.urgencyMedium",
  "4": "glpi.urgencyHigh",
  "5": "glpi.urgencyVeryHigh",
};
const IMPACT_VALUES = ["1", "2", "3", "4", "5"];
const IMPACT_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.impactVeryLow",
  "2": "glpi.impactLow",
  "3": "glpi.impactMedium",
  "4": "glpi.impactHigh",
  "5": "glpi.impactVeryHigh",
};
const PRIORITY_VALUES = ["1", "2", "3", "4", "5", "6"];
const PRIORITY_LABEL_KEYS: Record<string, string> = {
  "1": "glpi.priorityVeryLow",
  "2": "glpi.priorityLow",
  "3": "glpi.priorityMedium",
  "4": "glpi.priorityHigh",
  "5": "glpi.priorityVeryHigh",
  "6": "glpi.priorityMajor",
};
