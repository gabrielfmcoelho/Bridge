"use client";

import { useState } from "react";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import { ICON_PATHS, NAV_ICONS } from "@/lib/icon-paths";
import Button from "@/components/ui/Button";
import Drawer from "@/components/ui/Drawer";
import Icon from "@/components/ui/Icon";
import IconButton from "@/components/ui/IconButton";
import PillButton from "@/components/ui/PillButton";
import SectionCard from "@/components/ui/SectionCard";
import SectionHeading from "@/components/ui/SectionHeading";
import Toggle from "@/components/ui/Toggle";
import {
  PERSONAS, WIDGETS, canSeeWidget, columnOf, defaultWidgets, isMovable, newPrefs, suggestPersonas,
  type HomePrefs, type Persona, type WidgetId,
} from "./homeModel";

const PERSONA_ICONS: Record<Persona, string> = {
  infra: NAV_ICONS.Server,
  dev: ICON_PATHS.code,
  chamados: NAV_ICONS.Ticket,
  suporte: ICON_PATHS.clipboard,
  atlas: NAV_ICONS.Database,
};

const toggleIn = <T,>(list: T[], v: T) => (list.includes(v) ? list.filter((x) => x !== v) : [...list, v]);

/** First visit: "how do you use Bridge?" — personas pre-checked from the
 *  user's permissions. Both buttons finish onboarding; skipping keeps the
 *  suggestion. */
export function Onboarding({ onDone }: { onDone: (p: HomePrefs) => void }) {
  const { t } = useLocale();
  const { user } = useAuth();
  const suggested = suggestPersonas(user?.permissions ?? [], user?.role);
  const [picked, setPicked] = useState<Persona[]>(suggested);
  return (
    <SectionCard
      title={t("home.onboarding.title")}
      description={t("home.onboarding.description")}
      footer={
        <div className="flex items-center justify-end gap-2">
          <Button size="sm" variant="ghost" onClick={() => onDone(newPrefs(suggested))}>{t("home.onboarding.skip")}</Button>
          <Button size="sm" disabled={!picked.length} onClick={() => onDone(newPrefs(PERSONAS.filter((p) => picked.includes(p))))}>
            {t("home.onboarding.start")}
          </Button>
        </div>
      }
    >
      <div role="group" aria-label={t("home.personas.label")} className="grid grid-cols-1 sm:grid-cols-2 gap-2">
        {PERSONAS.map((p) => {
          const on = picked.includes(p);
          return (
            <button
              key={p}
              type="button"
              aria-pressed={on}
              onClick={() => setPicked((cur) => toggleIn(cur, p))}
              className={`group flex items-start gap-3 rounded-[var(--radius-md)] border px-3.5 py-3 text-left transition duration-150 active:scale-[0.99] ${
                on
                  ? "border-[var(--accent)]/50 bg-[var(--accent)]/[0.06]"
                  : "border-[var(--border-default)] hover:bg-[var(--bg-elevated)]"
              }`}
            >
              <span className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-md)] border ${
                on ? "border-[var(--accent)]/40 text-[var(--accent)]" : "border-[var(--border-subtle)] text-[var(--text-muted)]"
              }`}>
                <Icon path={PERSONA_ICONS[p]} className="w-4 h-4" strokeWidth={1.5} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block text-sm font-medium text-[var(--text-primary)]">{t(`home.personas.${p}`)}</span>
                <span className="block text-xs text-[var(--text-muted)] leading-relaxed">{t(`home.personas.${p}Hint`)}</span>
              </span>
              <span aria-hidden className={`mt-1 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border transition ${
                on ? "border-[var(--accent)] bg-[var(--accent)] text-[var(--bg-surface)]" : "border-[var(--border-default)]"
              }`}>
                {on && <Icon path={ICON_PATHS.check} className="w-3 h-3" strokeWidth={2.5} />}
              </span>
            </button>
          );
        })}
      </div>
    </SectionCard>
  );
}

/** "Personalizar": personas, then which widgets show and in what order.
 *  Every change saves at once. Changing personas restores that set's defaults. */
export function CustomizeDrawer({ open, onClose, prefs, onChange }: {
  open: boolean;
  onClose: () => void;
  prefs: HomePrefs;
  onChange: (p: HomePrefs) => void;
}) {
  const { t } = useLocale();
  const { user } = useAuth();
  const visible = (id: WidgetId) => canSeeWidget(id, user?.permissions ?? [], user?.role);
  const enabled = prefs.widgets.filter(visible);
  const disabled = WIDGETS.map((w) => w.id).filter((id) => visible(id) && !enabled.includes(id));

  const setPersonas = (personas: Persona[]) => onChange({ ...prefs, personas, widgets: defaultWidgets(personas) });
  // Widgets the user can't see now (no permission) keep their place in the
  // stored list, so they come back if the permission is granted later.
  const setWidgets = (widgets: WidgetId[]) => onChange({ ...prefs, widgets: [...widgets, ...prefs.widgets.filter((id) => !visible(id))] });
  const move = (id: WidgetId, by: -1 | 1) => {
    // Order matters within a column only, so step over the other column's ids.
    const col = columnOf(id, prefs);
    const same = enabled.filter((w) => columnOf(w, prefs) === col);
    const j = same.indexOf(id) + by;
    if (j < 0 || j >= same.length) return;
    const a = enabled.indexOf(id), b = enabled.indexOf(same[j]);
    const next = [...enabled];
    [next[a], next[b]] = [next[b], next[a]];
    setWidgets(next);
  };

  const row = (id: WidgetId, on: boolean) => (
    <li key={id} className="flex items-center gap-3 py-2">
      <Toggle checked={on} onChange={(v) => setWidgets(v ? [...enabled, id] : enabled.filter((w) => w !== id))} ariaLabel={t(`home.w.${id}`)} />
      <span className="flex-1 min-w-0">
        <span className={`block text-sm ${on ? "text-[var(--text-primary)]" : "text-[var(--text-muted)]"}`}>{t(`home.w.${id}`)}</span>
        {isMovable(id) ? (
          <span role="group" aria-label={t("home.customize.column")} className="mt-1 flex gap-1">
            {(["main", "side"] as const).map((col) => (
              <PillButton key={col} size="sm" active={columnOf(id, prefs) === col} onClick={() => onChange({ ...prefs, placement: { ...prefs.placement, [id]: col } })}>
                {t(`home.column.${col}`)}
              </PillButton>
            ))}
          </span>
        ) : (
          <span className="block text-2xs text-[var(--text-muted)]">{t(`home.column.${columnOf(id, prefs)}`)}</span>
        )}
      </span>
      {on && (
        <span className="flex gap-1">
          <IconButton label={t("home.customize.moveUp")} onClick={() => move(id, -1)}><Icon path={ICON_PATHS.chevronUp} className="w-3.5 h-3.5" /></IconButton>
          <IconButton label={t("home.customize.moveDown")} onClick={() => move(id, 1)}><Icon path={ICON_PATHS.chevronDown} className="w-3.5 h-3.5" /></IconButton>
        </span>
      )}
    </li>
  );

  return (
    <Drawer
      open={open}
      onClose={onClose}
      side="right"
      title={t("home.customize.title")}
      footer={
        <div className="flex items-center justify-between gap-2">
          <Button size="sm" variant="ghost" onClick={() => setPersonas(prefs.personas)}>{t("home.customize.reset")}</Button>
          <Button size="sm" variant="secondary" onClick={onClose}>{t("common.close")}</Button>
        </div>
      }
    >
      <section className="space-y-2">
        <SectionHeading variant="label" as="h3">{t("home.personas.label")}</SectionHeading>
        <div className="flex flex-wrap gap-1.5">
          {PERSONAS.map((p) => (
            <PillButton
              key={p}
              active={prefs.personas.includes(p)}
              onClick={() => setPersonas(PERSONAS.filter((x) => (x === p) !== prefs.personas.includes(x)))}
              lead={<Icon path={PERSONA_ICONS[p]} className="w-3.5 h-3.5" strokeWidth={1.5} />}
            >
              {t(`home.personas.${p}`)}
            </PillButton>
          ))}
        </div>
        <p className="text-2xs text-[var(--text-muted)]">{t("home.customize.personasHint")}</p>
      </section>
      <section className="mt-6">
        <SectionHeading variant="label" as="h3">{t("home.customize.widgets")}</SectionHeading>
        <ul className="divide-y divide-[var(--border-subtle)]">
          {enabled.map((id) => row(id, true))}
          {disabled.map((id) => row(id, false))}
        </ul>
      </section>
    </Drawer>
  );
}
