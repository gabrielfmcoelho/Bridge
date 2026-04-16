"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { serviceCredentialsAPI, servicesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { ServiceCredential } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";

export default function ServiceCredentialsPage() {
  const { t } = useLocale();

  const { data: servicesWithCreds = [], isLoading } = useQuery({
    queryKey: ["service-credentials-all"],
    queryFn: serviceCredentialsAPI.listAll,
  });

  return (
    <PageShell>
      <PageHeader title={t("serviceCredentials.title")} />
      <p className="text-sm text-[var(--text-muted)] -mt-2 mb-6">{t("serviceCredentials.subtitle")}</p>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : servicesWithCreds.length === 0 ? (
        <EmptyState
          icon="key"
          title={t("serviceCredentials.noServices")}
          description={t("serviceCredentials.noServicesDesc")}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {servicesWithCreds.map((svc, i) => (
            <div key={svc.service_id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <ServiceCredentialCard
                serviceId={svc.service_id}
                nickname={svc.service_nickname}
                serviceType={svc.service_type}
                credentials={svc.credentials}
              />
            </div>
          ))}
        </div>
      )}
    </PageShell>
  );
}

function ServiceCredentialCard({
  serviceId,
  nickname,
  serviceType,
  credentials,
}: {
  serviceId: number;
  nickname: string;
  serviceType: string;
  credentials: ServiceCredential[];
}) {
  const { t } = useLocale();

  return (
    <Card className="h-full">
      <div className="flex items-start gap-3">
        <div className="w-10 h-10 rounded-[var(--radius-md)] bg-amber-500/10 border border-amber-500/20 flex items-center justify-center shrink-0">
          <svg className="w-5 h-5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z" />
          </svg>
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="font-semibold text-[var(--text-primary)] text-sm">{nickname}</h3>
          <div className="flex items-center gap-2 mt-0.5">
            {serviceType && <Badge color="amber">{serviceType}</Badge>}
            <span className="text-xs text-[var(--text-faint)]">
              {credentials.length} {t("serviceCredentials.credentials")}
            </span>
          </div>
        </div>
      </div>

      <div className="mt-4 pt-3 border-t border-[var(--border-subtle)] space-y-2">
        {credentials.map((cred) => (
          <CredentialRow key={cred.id} serviceId={serviceId} credential={cred} />
        ))}
      </div>
    </Card>
  );
}

function CredentialRow({ serviceId, credential }: { serviceId: number; credential: ServiceCredential }) {
  const { t } = useLocale();
  const [revealed, setRevealed] = useState(false);
  const [value, setValue] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const reveal = async () => {
    if (revealed) {
      setRevealed(false);
      setValue(null);
      return;
    }
    setLoading(true);
    try {
      const data = await servicesAPI.getCredential(serviceId, credential.id);
      setValue(data.credentials || "");
      setRevealed(true);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-[var(--bg-elevated)] rounded-[var(--radius-md)] border border-[var(--border-subtle)] p-2.5">
      <div className="flex items-center justify-between">
        <span className="text-xs font-medium text-[var(--text-primary)]">{credential.role_name}</span>
        <button
          onClick={reveal}
          disabled={loading}
          className="text-xs text-[var(--accent)] hover:underline disabled:opacity-50"
        >
          {loading ? t("common.loading") : revealed ? t("serviceCredentials.hide") : t("serviceCredentials.reveal")}
        </button>
      </div>
      {revealed && value !== null && (
        <pre className="mt-2 text-xs text-[var(--text-secondary)] bg-[var(--bg-surface)] rounded-[var(--radius-sm)] p-2 overflow-x-auto whitespace-pre-wrap break-all border border-[var(--border-subtle)]">
          {value}
        </pre>
      )}
    </div>
  );
}
