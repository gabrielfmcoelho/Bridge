"use client";

import Badge from "@/components/ui/Badge";
import CopyButton from "@/components/ui/CopyButton";
import Divider from "@/components/ui/Divider";
import Field from "@/components/ui/Field";
import SectionHeading from "@/components/ui/SectionHeading";
import { useLocale } from "@/contexts/LocaleContext";
import type { BundleKeyInfo } from "@/lib/types";

// ShareKeyScopes shows, under a shared API access key on the public reveal
// page, what the guest needs to get a token: the client id, the API, the
// token URL and the scopes the key carries (ready-to-paste `scope=` value).
export default function ShareKeyScopes({ info }: { info: BundleKeyInfo }) {
  const { t } = useLocale();
  const scopeParam = info.scopes.map((s) => s.name).join(" ");
  return (
    <div className="mt-4 space-y-4">
      <Divider />
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <Field label={t("share.key.clientId")} value={info.client_id ?? ""} mono />
        <Field label={t("share.key.api")} value={info.api_name} />
        {info.api_base_url && <Field label={t("share.key.baseUrl")} value={info.api_base_url} mono />}
        {info.rate_limit_per_minute ? (
          <Field label={t("share.key.rateLimit")} value={t("share.key.rateLimitValue", { count: String(info.rate_limit_per_minute) })} />
        ) : null}
      </div>
      {info.token_url && (
        <div className="flex items-end gap-2">
          <Field label={t("share.key.tokenUrl")} value={info.token_url} mono className="min-w-0 flex-1" />
          <CopyButton value={info.token_url} size="sm" icon />
        </div>
      )}
      {info.scopes.length > 0 && (
        <div>
          <SectionHeading variant="rule" count={info.scopes.length} hint={t("share.key.hint")}>
            {t("share.key.title")}
          </SectionHeading>
          <ul className="space-y-2">
            {info.scopes.map((s) => (
              <li key={s.name} className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                <span className="font-mono text-xs text-[var(--text-primary)]">{s.name}</span>
                {s.kind && (
                  <Badge color={s.kind === "route" ? "info" : "purple"}>
                    {t(s.kind === "route" ? "share.key.kindRoute" : "share.key.kindModifier")}
                  </Badge>
                )}
                <span className="basis-full text-xs text-[var(--text-muted)]">
                  {s.description || t("share.key.noDescription")}
                </span>
              </li>
            ))}
          </ul>
          <div className="mt-3 flex items-end gap-2">
            <Field label={t("share.key.scopeParam")} value={scopeParam} mono className="min-w-0 flex-1" />
            <CopyButton value={scopeParam} size="sm" icon />
          </div>
        </div>
      )}
    </div>
  );
}
