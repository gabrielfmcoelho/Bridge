"use client";

import Link from "next/link";
import { useLocale } from "@/contexts/LocaleContext";
import Drawer from "@/components/ui/Drawer";
import Badge from "@/components/ui/Badge";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import StatusDot from "@/components/ui/StatusDot";
import { RowList, ListRow, RowText } from "@/components/ui/RowList";
import { ICON_PATHS } from "@/lib/icon-paths";
import type { DatabaseInstance } from "@/lib/types";

/** The engines the scan or Coolify found; "+" registers a database on one.
 *  Unregistered ones come first: they are the gap this drawer is for. */
export default function InstancesDrawer({ open, onClose, instances, canEdit, onRegister }: {
  open: boolean;
  onClose: () => void;
  instances: DatabaseInstance[];
  canEdit: boolean;
  onRegister: (inst: DatabaseInstance) => void;
}) {
  const { t } = useLocale();
  const sorted = [...instances].sort((a, b) => Number(a.registered_count > 0) - Number(b.registered_count > 0) || a.nickname.localeCompare(b.nickname));
  return (
    <Drawer open={open} onClose={onClose} title={t("atlas.databases.instances.title")}>
      <p className="text-sm text-[var(--text-secondary)] mb-4">{t("atlas.databases.instances.hint")}</p>
      <RowList>
        {sorted.map((i) => (
          <ListRow key={i.id} href={`/services/${i.id}`}>
            <StatusDot color={i.container_status === "offline" ? "muted" : "success"} title={i.container_status || undefined} />
            <RowText title={i.nickname} meta={[i.host_nickname, i.container_image || i.service_subtype].filter(Boolean).join(" · ") || undefined} />
            <Badge color="cyan">{t(`atlas.databases.engine.${i.engine}`)}</Badge>
            {i.registered_count > 0
              ? <span className="shrink-0 text-xs text-[var(--text-muted)]">{t("atlas.databases.instances.registered", { count: String(i.registered_count) })}</span>
              : <Badge color="amber">{t("atlas.databases.instances.unregistered")}</Badge>}
            {canEdit && (
              <IconButton label={t("atlas.databases.add")} onClick={(e) => { e.preventDefault(); e.stopPropagation(); onRegister(i); }}>
                <Icon path={ICON_PATHS.plus} />
              </IconButton>
            )}
          </ListRow>
        ))}
      </RowList>
      {instances.length === 0 && <p className="text-sm text-[var(--text-muted)]">{t("atlas.databases.instances.none")}</p>}
      <p className="mt-4 text-xs text-[var(--text-muted)]">
        {t("atlas.databases.instances.footer")} <Link href="/services" className="text-[var(--accent)] hover:underline">{t("nav.services")}</Link>
      </p>
    </Drawer>
  );
}
