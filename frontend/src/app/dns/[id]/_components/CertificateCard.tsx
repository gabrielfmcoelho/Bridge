"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { dnsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { certDaysLeft } from "@/lib/dnsCert";
import SectionHeading from "@/components/ui/SectionHeading";
import Card from "@/components/ui/Card";
import Field from "@/components/ui/Field";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";
import FormError from "@/components/ui/FormError";
import CertBadge from "../../_components/CertBadge";
import type { DNSRecord } from "@/lib/types";

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

  if (!dns.has_https && !dns.cert_checked_at) return null;

  const exp = dns.cert_expires_at;
  const days = exp ? certDaysLeft(exp) : -1;
  const expires = exp ? (days >= 0 ? `${formatDate(exp)} · ${t("dns.certDaysLeft", { days: String(days) })}` : formatDate(exp)) : "";

  return (
    <>
      <SectionHeading
        actions={
          <>
            <CertBadge dns={dns} />
            {canEdit && (
              <Button size="sm" variant="secondary" loading={rescan.isPending} onClick={() => rescan.mutate()}>
                {t("dns.certRescan")}
              </Button>
            )}
          </>
        }
      >
        {t("dns.certificate")}
      </SectionHeading>
      <Card hover={false}>
        <div className="space-y-4">
          {rescan.isError && <FormError message={rescan.error.message} />}
          {dns.cert_error && <StatusAlert variant={exp ? "warning" : "error"}>{dns.cert_error}</StatusAlert>}
          <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4 text-sm">
            <Field label={t("dns.certExpires")} value={expires} />
            <Field label={t("dns.certIssued")} value={dns.cert_not_before ? formatDate(dns.cert_not_before) : ""} />
            <Field label={t("dns.certLastScan")} value={dns.cert_checked_at ? formatDateTime(dns.cert_checked_at) : ""} />
            <Field label={t("dns.certIssuer")} value={dns.cert_issuer || ""} />
            <Field label={t("dns.certSubject")} value={dns.cert_subject || ""} />
            <Field label={t("dns.certSans")} value={dns.cert_sans || ""} mono />
          </div>
        </div>
      </Card>
    </>
  );
}
