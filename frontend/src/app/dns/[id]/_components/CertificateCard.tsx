"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { certDaysLeft, certState, certTone, certValidityPct } from "@/lib/dnsCert";
import SectionCard from "@/components/ui/SectionCard";
import Field from "@/components/ui/Field";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import { CardMeter } from "@/components/inventory";
import { ICON_PATHS } from "@/lib/icon-paths";
import { certLabel } from "../../_components/CertBadge";
import type { DNSRecord } from "@/lib/types";

/** The observed side of a DNS record: its TLS certificate as last scanned.
 *  Always present — a domain without HTTPS says so in the empty line. */
export default function CertificateCard({ dns, canEdit }: { dns: DNSRecord; canEdit: boolean }) {
  const { t, formatDate, formatDateTime } = useLocale();
  const queryClient = useQueryClient();
  const rescan = useMutation({
    mutationFn: () => dnsAPI.scanCert(dns.id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["dns"] });
      queryClient.invalidateQueries({ queryKey: ["dns-table"] });
    },
  });

  const state = certState(dns);
  const exp = dns.cert_expires_at;
  const days = exp ? certDaysLeft(exp) : null;
  const tone = certTone(state);
  const empty = state === "none" ? t("dns.certNone") : state === "unscanned" ? t("dns.certUnscannedHint") : undefined;

  return (
    <SectionCard
      as="h3"
      title={t("dns.certificate")}
      description={certLabel(dns, t) || undefined}
      empty={empty}
      controls={canEdit && dns.has_https ? (
        <IconButton onClick={() => rescan.mutate()} disabled={rescan.isPending} label={t("dns.certRescan")}>
          <Icon path={ICON_PATHS.refresh} className={rescan.isPending ? "animate-spin" : ""} />
        </IconButton>
      ) : undefined}
    >
      <div className="space-y-4">
        {rescan.isError && <FormError message={rescan.error.message} />}
        {dns.cert_error && <StatusAlert variant={exp ? "warning" : "error"}>{dns.cert_error}</StatusAlert>}
        <CardMeter
          label={t("dns.certValidity")}
          pct={certValidityPct(dns)}
          reading={days !== null ? t("dns.daysLeft", { days: String(Math.max(days, 0)) }) : undefined}
          tone={tone === "danger" ? "danger" : tone === "warning" ? "warning" : "success"}
          caption={exp ? formatDate(exp) : undefined}
        />
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
          <Field label={t("dns.certIssued")} value={dns.cert_not_before ? formatDate(dns.cert_not_before) : ""} />
          <Field label={t("dns.certLastScan")} value={dns.cert_checked_at ? formatDateTime(dns.cert_checked_at) : ""} />
          <Field label={t("dns.certIssuer")} value={dns.cert_issuer || ""} />
          <Field label={t("dns.certSubject")} value={dns.cert_subject || ""} />
          <Field className="sm:col-span-2" label={t("dns.certSans")} value={dns.cert_sans || ""} mono />
        </div>
      </div>
    </SectionCard>
  );
}
