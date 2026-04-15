"use client";

import { useRouter } from "next/navigation";
import Card from "@/components/ui/Card";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import type { ServiceCredential } from "@/lib/types";

interface CredentialsTabProps {
  credentials: ServiceCredential[];
  serviceId: number;
  isAdmin: boolean;
  t: (key: string) => string;
}

export default function CredentialsTab({ credentials, serviceId, isAdmin, t }: CredentialsTabProps) {
  const router = useRouter();

  return (
    <div className="space-y-5 animate-fade-in">
      <Card hover={false}>
        <h2
          className="text-sm font-semibold text-[var(--text-secondary)] mb-3"
          style={{ fontFamily: "var(--font-display)" }}
        >
          {t("service.credentials")}
        </h2>
        {credentials && credentials.length > 0 ? (
          <div className="space-y-2">
            {credentials.map((cred) => (
              <div
                key={cred.id}
                className="flex items-center gap-2 text-sm p-2 rounded-[var(--radius-md)] bg-[var(--bg-elevated)]"
              >
                <svg className="w-4 h-4 text-purple-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
                </svg>
                <Badge>{cred.role_name}</Badge>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-[var(--text-faint)]">No credentials configured</p>
        )}
        {isAdmin && (
          <div className="mt-4">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => router.push(`/services/${serviceId}/credentials/new`)}
            >
              Add Credential
            </Button>
          </div>
        )}
      </Card>
    </div>
  );
}
