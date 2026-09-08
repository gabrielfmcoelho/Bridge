"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import Drawer from "@/components/ui/Drawer";
import DrawerSection from "@/components/ui/DrawerSection";
import Select from "@/components/ui/Select";
import Button from "@/components/ui/Button";
import { offeringsAPI, enumsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { RequestStatus } from "@/lib/types";

const STATUSES: RequestStatus[] = [
  "submitted", "under_review", "approved", "in_progress",
  "delivered", "rejected", "cancelled", "needs_info",
];

interface RequestFilterDrawerProps {
  open: boolean;
  onClose: () => void;
  status: string;
  onStatusChange: (v: string) => void;
  offeringId: string;
  onOfferingIdChange: (v: string) => void;
  priority: string;
  onPriorityChange: (v: string) => void;
  onClearAll: () => void;
}

export default function RequestFilterDrawer({
  open, onClose, status, onStatusChange, offeringId, onOfferingIdChange, priority, onPriorityChange, onClearAll,
}: RequestFilterDrawerProps) {
  const { t } = useLocale();
  const [openSection, setOpenSection] = useState<string | null>("status");
  const toggle = (key: string) => setOpenSection((prev) => (prev === key ? null : key));

  const { data: offerings = [] } = useQuery({
    queryKey: ["offerings", "active"],
    queryFn: () => offeringsAPI.list({ active: true }),
    enabled: open,
  });
  const { data: priorityEnum = [] } = useQuery({
    queryKey: ["enums", "issue_priority"],
    queryFn: () => enumsAPI.list("issue_priority"),
    enabled: open,
  });

  const statusOptions = STATUSES.map((s) => ({ value: s, label: t(`requests.status.${s}`) }));
  const offeringOptions = offerings.map((o) => ({ value: String(o.id), label: o.name }));
  const priorityOptions = priorityEnum.map((o) => ({ value: o.value, label: t(`requests.priorityLevels.${o.value}`) }));

  const activeCount = [status, offeringId, priority].filter(Boolean).length;

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={t("requests.filters.title")}
      footer={
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" className="flex-1" onClick={onClearAll} disabled={activeCount === 0}>
            {t("filters.clearAll")}
          </Button>
          <Button size="sm" className="flex-1" onClick={onClose}>
            {t("filters.apply")}
          </Button>
        </div>
      }
    >
      <div className="space-y-0">
        <DrawerSection title={t("common.status")} open={openSection === "status"} onToggle={() => toggle("status")} active={!!status}>
          <Select value={status} onChange={(e) => onStatusChange(e.target.value)} options={statusOptions} />
        </DrawerSection>

        <DrawerSection title={t("requests.offering")} open={openSection === "offering"} onToggle={() => toggle("offering")} active={!!offeringId}>
          <Select
            value={offeringId}
            onChange={(e) => onOfferingIdChange(e.target.value)}
            options={offeringOptions}
            searchPlaceholder={t("requests.offering")}
          />
        </DrawerSection>

        <DrawerSection title={t("requests.priority")} open={openSection === "priority"} onToggle={() => toggle("priority")} active={!!priority}>
          <Select value={priority} onChange={(e) => onPriorityChange(e.target.value)} options={priorityOptions} />
          {/* ponytail: requestsAPI.listPaginated has no priority param — this
              narrows only the page already fetched from the server, not the
              whole dataset. Said out loud here rather than left implicit,
              same ceiling as the table's client-side sort. */}
          <p className="text-xs text-[var(--text-faint)]">{t("requests.filters.priorityHint")}</p>
        </DrawerSection>
      </div>
    </Drawer>
  );
}
