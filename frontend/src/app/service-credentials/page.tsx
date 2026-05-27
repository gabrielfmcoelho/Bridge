"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { secretsAPI, servicesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import type { Secret } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";

// ServiceCredentialsPage groups shared service-scoped secrets by parent
// service. The unified vault (/api/secrets) doesn't ship a server-side
// group-by-service shape (the old serviceCredentialsAPI.listAll did), so
// this page joins two queries on the client: all shared service secrets
// + the services list (for nicknames + types).
export default function ServiceCredentialsPage() {
  const { t } = useLocale();

  const { data: secrets = [], isLoading: secretsLoading } = useQuery({
    queryKey: ["secrets", "service", "shared"],
    queryFn: () => secretsAPI.list({ scope: "service", visibility: "shared" }),
  });

  const { data: services = [], isLoading: servicesLoading } = useQuery({
    queryKey: ["services-list"],
    queryFn: servicesAPI.list,
  });

  const isLoading = secretsLoading || servicesLoading;

  const grouped = useMemo(() => {
    const byID = new Map<number, { service: typeof services[number]; secrets: Secret[] }>();
    for (const s of services) {
      byID.set(s.id, { service: s, secrets: [] });
    }
    for (const sec of secrets) {
      if (sec.parent_id == null) continue;
      const bucket = byID.get(sec.parent_id);
      if (bucket) bucket.secrets.push(sec);
    }
    return Array.from(byID.values()).filter((g) => g.secrets.length > 0);
  }, [secrets, services]);

  return (
    <PageShell>
      <PageHeader title={t("serviceCredentials.title")} />
      <p className="text-sm text-[var(--text-muted)] -mt-2 mb-6">{t("serviceCredentials.subtitle")}</p>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : grouped.length === 0 ? (
        <EmptyState
          icon="key"
          title={t("serviceCredentials.noServices")}
          description={t("serviceCredentials.noServicesDesc")}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {grouped.map((g, i) => (
            <div key={g.service.id} className={`animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
              <ServiceCredentialCard
                nickname={g.service.nickname}
                serviceType={g.service.service_type || ""}
                secrets={g.secrets}
              />
            </div>
          ))}
        </div>
      )}
    </PageShell>
  );
}

function ServiceCredentialCard({
  nickname,
  serviceType,
  secrets,
}: {
  nickname: string;
  serviceType: string;
  secrets: Secret[];
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
              {secrets.length} {t("serviceCredentials.credentials")}
            </span>
          </div>
        </div>
      </div>

      <div className="mt-4 pt-3 border-t border-[var(--border-subtle)] space-y-2">
        {secrets.map((s) => (
          <SecretRow key={s.id} secret={s} />
        ))}
      </div>
    </Card>
  );
}

function SecretRow({ secret }: { secret: Secret }) {
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
      const data = await secretsAPI.reveal(secret.id);
      setValue(data.payload || "");
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
        <span className="text-xs font-medium text-[var(--text-primary)]">{secret.name}</span>
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
