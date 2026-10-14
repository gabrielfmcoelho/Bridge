"use client";

import Link from "next/link";
import { useState } from "react";
import { useAuth } from "@/contexts/AuthContext";
import { useLocale } from "@/contexts/LocaleContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import PageShell from "@/components/layout/PageShell";
import Button from "@/components/ui/Button";
import Heading from "@/components/ui/Heading";
import Icon from "@/components/ui/Icon";
import PageHeader from "@/components/ui/PageHeader";
import SectionCard from "@/components/ui/SectionCard";
import StatusAlert from "@/components/ui/StatusAlert";
import { Skeleton } from "@/components/ui/Skeleton";
import { canSeeWidget, columnOf, pendingSteps, type Column, type HomePrefs, type WidgetId } from "./_home/homeModel";
import { useHomePrefs } from "./_home/useHomePrefs";
import { WIDGET_COMPONENTS } from "./_home/widgets";
import { CustomizeDrawer, Onboarding } from "./_home/HomeSetup";

/**
 * Início: the user's starting point. A greeting, onboarding on the first
 * visit, then the widgets for what they do here (personas), saved per user in
 * users.home_prefs. Host numbers live on the Hosts Dashboard tab.
 */
export default function HomePage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const { prefs, loading, error, save } = useHomePrefs();
  const [customizing, setCustomizing] = useState(false);

  const firstName = user?.display_name?.trim().split(/\s+/)[0] || user?.username;
  const entidade = user?.entidades?.find((e) => e.is_primary) ?? user?.entidades?.[0];

  const column = (col: Column) =>
    prefs ? prefs.widgets.filter((id) => columnOf(id, prefs) === col && canSeeWidget(id, user?.permissions ?? [], user?.role)) : [];

  return (
    <PageShell>
      <PageHeader
        title={t("nav.home")}
        actions={prefs && (
          <Button size="sm" variant="secondary" onClick={() => setCustomizing(true)}>
            <Icon path={ICON_PATHS.gear} className="w-3.5 h-3.5" />
            {t("home.customize.open")}
          </Button>
        )}
      />

      {user && (
        <section className="mb-6 stagger-in">
          <Heading as="h2" size="xl">
            {t("home.welcome.greeting")} <span className="font-normal">{firstName}</span>
          </Heading>
          <p className="mt-1 text-sm text-[var(--text-muted)]">
            {[entidade?.name, user.role].filter(Boolean).join(" · ")}
          </p>
        </section>
      )}

      {error ? (
        <StatusAlert variant="error">{t("home.loadFailed", { message: error.message })}</StatusAlert>
      ) : loading || !user ? (
        <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4" aria-busy>
          <Skeleton className="h-64" />
          <Skeleton className="h-48" />
        </div>
      ) : !prefs ? (
        <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4 items-start">
          <div className="stagger-in"><Onboarding onDone={save} /></div>
          <div className="stagger-in" style={{ "--i": 1 } as React.CSSProperties}>
            <WelcomeNote />
          </div>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4 items-start">
            <WidgetColumn ids={column("main")} prefs={prefs} offset={0} />
            <div className="space-y-4">
              <FirstSteps prefs={prefs} onChange={save} />
              <WidgetColumn ids={column("side")} prefs={prefs} offset={1} />
            </div>
          </div>
          {!prefs.widgets.length && (
            <p className="mt-4 text-sm text-[var(--text-muted)]">{t("home.noWidgets")}</p>
          )}
          <CustomizeDrawer open={customizing} onClose={() => setCustomizing(false)} prefs={prefs} onChange={save} />
        </>
      )}
    </PageShell>
  );
}

function WidgetColumn({ ids, prefs, offset }: { ids: WidgetId[]; prefs: HomePrefs; offset: number }) {
  if (!ids.length) return null;
  return (
    <div className="space-y-4 min-w-0">
      {ids.map((id, i) => {
        const W = WIDGET_COMPONENTS[id];
        return (
          <div key={id} className="stagger-in" style={{ "--i": i * 2 + offset } as React.CSSProperties}>
            <W personas={prefs.personas} />
          </div>
        );
      })}
    </div>
  );
}

/** Beside the onboarding card: what Início is, in two lines. */
function WelcomeNote() {
  const { t } = useLocale();
  return (
    <SectionCard title={t("home.welcome.title")}>
      <p className="text-sm text-[var(--text-secondary)] leading-relaxed max-w-[65ch]">{t("home.welcome.body")}</p>
    </SectionCard>
  );
}

/** Per-persona first steps, five at a time. Opening one marks it done; "Dispensar" hides them all. */
function FirstSteps({ prefs, onChange }: { prefs: HomePrefs; onChange: (p: HomePrefs) => void }) {
  const { t } = useLocale();
  const pending = pendingSteps(prefs.personas, prefs.done_steps);
  if (!pending.length) return null;
  // A few at a time: four personas would list eleven. The next ones show as these get done.
  const steps = pending.slice(0, 5);
  const done = (ids: string[]) => onChange({ ...prefs, done_steps: [...prefs.done_steps, ...ids] });
  return (
    <SectionCard
      as="h3"
      title={t("home.steps.title")}
      icon={ICON_PATHS.sparkles}
      body="flush"
      controls={
        <Button size="sm" variant="ghost" onClick={() => done(pending.map((s) => s.id))}>{t("home.steps.dismiss")}</Button>
      }
    >
      <ol className="divide-y divide-[var(--border-subtle)]">
        {steps.map((s, i) => (
          <li key={s.id}>
            <Link
              href={s.href}
              onClick={() => done([s.id])}
              className="flex items-center gap-3 px-5 py-2.5 hover:bg-[var(--bg-elevated)] transition-colors duration-100"
            >
              <span className="flex h-5 w-5 shrink-0 items-center justify-center rounded-full border border-[var(--border-default)] font-mono text-2xs text-[var(--text-muted)]">{i + 1}</span>
              <span className="flex-1 text-sm text-[var(--text-primary)]">{t(s.labelKey)}</span>
              <Icon path={ICON_PATHS.chevronRight} className="w-3.5 h-3.5 text-[var(--text-muted)]" />
            </Link>
          </li>
        ))}
      </ol>
    </SectionCard>
  );
}
