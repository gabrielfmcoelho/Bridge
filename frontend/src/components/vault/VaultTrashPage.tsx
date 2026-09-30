"use client";

import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { secretsAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import PageShell from "@/components/layout/PageShell";
import PageHeader from "@/components/ui/PageHeader";
import SectionCard from "@/components/ui/SectionCard";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { SkeletonCard } from "@/components/ui/Skeleton";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import { getTimeAgo } from "@/lib/utils";
import { scopeLabel } from "./SecretDetailDrawer";
import { TYPE_ICON } from "./VaultTable";

// The vault trash (app/secrets/trash/page.tsx re-exports it): secrets the
// caller deleted or a parent's delete took along, restorable one by one.
export default function VaultTrashPage() {
  const { t, locale } = useLocale();
  const router = useRouter();
  const flag = useFlag();
  const qc = useQueryClient();
  const { data: items = [], isLoading } = useQuery({ queryKey: ["secrets-trash"], queryFn: secretsAPI.trash });
  const restore = useMutation({
    mutationFn: (id: number) => secretsAPI.restore(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["secrets-trash"] });
      qc.invalidateQueries({ queryKey: ["secrets-all"] });
    },
    onError: (e) => flag({ appearance: "error", title: t("vault.restore"), description: e instanceof Error ? e.message : t("form.saveFailed") }),
  });

  return (
    <PageShell>
      <PageHeader title={t("vault.trashTitle")} description={t("vault.trashDescription")}
        actions={<Button size="sm" variant="secondary" onClick={() => router.push("/secrets")}>{t("vault.backToVault")}</Button>} />
      {isLoading ? <SkeletonCard /> : (
        <SectionCard as="h2" title={t("vault.trashTitle")} count={items.length} body="flush"
          empty={items.length === 0 ? t("vault.trashEmpty") : undefined}>
          <RowList>
            {items.map((s) => (
              <ListRow key={s.id}>
                <Icon path={TYPE_ICON[s.type] ?? ICON_PATHS.key} className="w-3.5 h-3.5 shrink-0 text-[var(--text-muted)]" />
                <RowText title={s.name} meta={<>
                  <span>{t(`vault.type.${s.type}`)} · {scopeLabel(s, t)}</span>
                  {s.deleted_at && <span>{t("vault.deletedAgo", { ago: getTimeAgo(s.deleted_at, locale) })}</span>}
                </>} />
                <Button size="sm" variant="secondary" loading={restore.isPending && restore.variables === s.id}
                  onClick={() => restore.mutate(s.id)}>{t("vault.restore")}</Button>
              </ListRow>
            ))}
          </RowList>
        </SectionCard>
      )}
    </PageShell>
  );
}
