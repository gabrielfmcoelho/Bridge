"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import SectionCard from "@/components/ui/SectionCard";
import Field from "@/components/ui/Field";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import Lozenge from "@/components/ui/Lozenge";
import FormError from "@/components/ui/FormError";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { DNSRecord } from "@/lib/types";

const STATUS_APPEARANCE = { online: "success", no_content: "moved", error: "removed", offline: "removed" } as const;

/** The domain as last seen from outside: DNS answer and the HTTP status on
 *  80 and 443. The same scan fills CertificateCard. */
export default function ObservationCard({ dns, canEdit }: { dns: DNSRecord; canEdit: boolean }) {
  const { t, formatDateTime } = useLocale();
  const queryClient = useQueryClient();
  const rescan = useMutation({
    mutationFn: () => dnsAPI.scanCert(dns.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dns"] });
      queryClient.invalidateQueries({ queryKey: ["dns-table"] });
    },
  });

  const status = dns.obs_status || "";
  const code = (c?: number) => (c ? String(c) : t("dns.obsNoAnswer"));

  return (
    <SectionCard
      as="h3"
      title={t("dns.observation")}
      empty={dns.observed_at ? undefined : t("dns.obsUnscannedHint")}
      controls={canEdit ? (
        <IconButton onClick={() => rescan.mutate()} disabled={rescan.isPending} label={t("dns.obsRescan")}>
          <Icon path={ICON_PATHS.refresh} className={rescan.isPending ? "animate-spin" : ""} />
        </IconButton>
      ) : undefined}
    >
      <div className="space-y-4">
        {rescan.isError && <FormError message={rescan.error.message} />}
        {status && <Lozenge appearance={STATUS_APPEARANCE[status]}>{t(`dns.obsStatus.${status}`)}</Lozenge>}
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
          <Field label={t("dns.obsRecordType")} value={dns.obs_record_type || t("dns.obsUnresolved")} />
          <Field className="sm:col-span-2" label={t("dns.obsTarget")} value={dns.obs_target || ""} mono />
          <Field label="HTTP (80)" value={code(dns.obs_http_status)} mono />
          <Field label="HTTPS (443)" value={code(dns.obs_https_status)} mono />
          <Field label={t("dns.obsLastCheck")} value={dns.observed_at ? formatDateTime(dns.observed_at) : ""} />
        </div>
        <p className="text-xs text-[var(--text-muted)]">{t("dns.observationHint")}</p>
      </div>
    </SectionCard>
  );
}
