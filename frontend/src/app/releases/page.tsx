"use client";

import { useState, useMemo } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { releasesAPI, projectsAPI, issuesAPI } from "@/lib/api";
import { useLocale } from "@/contexts/LocaleContext";
import { useAuth } from "@/contexts/AuthContext";
import type { Release } from "@/lib/types";
import PageShell from "@/components/layout/PageShell";
import Card from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import IconButton from "@/components/ui/IconButton";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Drawer from "@/components/ui/Drawer";
import MarkdownEditor, { MarkdownContent } from "@/components/ui/MarkdownEditor";
import StepIndicator from "@/components/ui/StepIndicator";
import FormError from "@/components/ui/FormError";
import CheckboxList from "@/components/ui/CheckboxList";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";

const RELEASE_STATUSES = ["pending", "ongoing", "ready", "live", "canceled"] as const;

const statusColors: Record<string, string> = {
  pending: "bg-gray-500/15 text-gray-400 border-gray-500/30",
  ongoing: "bg-cyan-500/15 text-cyan-400 border-cyan-500/30",
  ready: "bg-amber-500/15 text-amber-400 border-amber-500/30",
  live: "bg-emerald-500/15 text-emerald-400 border-emerald-500/30",
  canceled: "bg-red-500/15 text-red-400 border-red-500/30",
};

const statusDots: Record<string, string> = {
  pending: "bg-gray-400",
  ongoing: "bg-cyan-400",
  ready: "bg-amber-400",
  live: "bg-emerald-400",
  canceled: "bg-red-400",
};

const timelineLineColors: Record<string, string> = {
  pending: "border-gray-500/40",
  ongoing: "border-cyan-500/40",
  ready: "border-amber-500/40",
  live: "border-emerald-500/40",
  canceled: "border-red-500/20",
};

export default function ReleasesPage() {
  const { t } = useLocale();
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const canEdit = user?.role === "admin" || user?.role === "editor";
  const [showForm, setShowForm] = useState(false);
  const [editRelease, setEditRelease] = useState<(Release & { issue_ids: number[] }) | null>(null);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [showFilters, setShowFilters] = useState(false);

  const { data: releases = [], isLoading } = useQuery({
    queryKey: ["releases"],
    queryFn: releasesAPI.list,
  });

  const { data: projects = [] } = useQuery({
    queryKey: ["projects"],
    queryFn: projectsAPI.list,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => releasesAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["releases"] }),
  });

  const projectMap = Object.fromEntries(projects.map((p) => [p.id, p.name]));

  const filteredReleases = useMemo(() => {
    return releases.filter((rel) => {
      const matchesSearch = !search || rel.title.toLowerCase().includes(search.toLowerCase()) ||
        (rel.description && rel.description.toLowerCase().includes(search.toLowerCase())) ||
        (rel.project_id && projectMap[rel.project_id] && projectMap[rel.project_id].toLowerCase().includes(search.toLowerCase()));
      const matchesStatus = !statusFilter || rel.status === statusFilter;
      return matchesSearch && matchesStatus;
    });
  }, [releases, search, statusFilter, projectMap]);

  return (
    <PageShell>
      {/* Header */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{t("release.title")}</h1>
        <div className="flex items-center gap-1.5">
          {/* Desktop add button */}
          {canEdit && (
            <div className="hidden sm:block">
              <Button size="sm" onClick={() => setShowForm(true)}>
                <span className="mr-1">+</span> {t("release.create")}
              </Button>
            </div>
          )}
          {/* Mobile: filter + add */}
          <IconButton
            variant={search || statusFilter ? "active" : "outline"}
            size="md"
            onClick={() => setShowFilters(true)}
            title={t("common.filter")}
            className="sm:hidden"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
            </svg>
          </IconButton>
          {canEdit && (
            <IconButton
              variant="accent"
              size="md"
              onClick={() => setShowForm(true)}
              title={t("release.create")}
              className="sm:hidden"
            >
              <svg className="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
              </svg>
            </IconButton>
          )}
        </div>
      </div>

      {/* Filters — desktop */}
      <div className="hidden sm:flex flex-wrap items-center gap-3 mb-5">
        <div className="relative max-w-xs flex-1 min-w-[200px]">
          <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            placeholder={t("common.search")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-1.5 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <button
            onClick={() => setStatusFilter("")}
            className={`px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-all duration-150 font-medium whitespace-nowrap ${
              !statusFilter
                ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-secondary)]"
            }`}
          >
            All
          </button>
          {RELEASE_STATUSES.map((status) => (
            <button
              key={status}
              onClick={() => setStatusFilter(status === statusFilter ? "" : status)}
              className={`px-3 py-1.5 text-xs rounded-[var(--radius-md)] border transition-all duration-150 font-medium whitespace-nowrap ${
                status === statusFilter
                  ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                  : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:bg-[var(--bg-overlay)] hover:text-[var(--text-secondary)]"
              }`}
            >
              {t(`release.${status}`)}
            </button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : filteredReleases.length === 0 ? (
        <EmptyState
          icon="box"
          title={t("common.noResults")}
          description={search || statusFilter ? "Try adjusting your filters" : t("release.noReleases")}
          action={canEdit && !search && !statusFilter ? (
            <Button size="sm" onClick={() => setShowForm(true)}>+ {t("release.create")}</Button>
          ) : undefined}
        />
      ) : (
        <div className="relative">
          {/* Timeline line */}
          <div className="absolute left-[19px] top-0 bottom-0 w-px bg-[var(--border-subtle)]" />

          <div className="space-y-4">
            {filteredReleases.map((rel, i) => (
              <div key={rel.id} className={`relative pl-12 animate-slide-up stagger-${Math.min(i + 1, 9)}`} style={{ animationFillMode: "both" }}>
                {/* Timeline dot */}
                <div className={`absolute left-[12px] top-5 w-[15px] h-[15px] rounded-full border-2 ${timelineLineColors[rel.status]} ${statusDots[rel.status]}`} />

                <Card
                  hover={canEdit}
                  className={`${rel.status === "canceled" ? "opacity-60" : ""}`}
                  onClick={canEdit ? () => setEditRelease(rel) : undefined}
                >
                  <div className="flex items-start justify-between gap-3 mb-2">
                    <h3 className="font-semibold text-[var(--text-primary)]">{rel.title}</h3>
                    <span className={`text-[10px] px-2 py-0.5 rounded-full border font-medium shrink-0 ${statusColors[rel.status]}`}>
                      {t(`release.${rel.status}`)}
                    </span>
                  </div>

                  {rel.description && (
                    <div className="mb-3">
                      <MarkdownContent content={rel.description} />
                    </div>
                  )}

                  <div className="flex items-center gap-3 text-xs text-[var(--text-muted)]">
                    {rel.project_id && projectMap[rel.project_id] && (
                      <span className="bg-[var(--bg-elevated)] px-2 py-0.5 rounded">{projectMap[rel.project_id]}</span>
                    )}
                    {rel.target_date && (
                      <span>Target: {rel.target_date}</span>
                    )}
                    {rel.live_date && (
                      <span className="text-emerald-400">Live: {rel.live_date}</span>
                    )}
                    {rel.issue_ids && rel.issue_ids.length > 0 && (
                      <span>{rel.issue_ids.length} {rel.issue_ids.length === 1 ? "issue" : "issues"}</span>
                    )}
                  </div>
                </Card>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Mobile filter drawer */}
      <div className="sm:hidden">
        <Drawer open={showFilters} onClose={() => setShowFilters(false)} title={t("common.filter")}>
          <div className="space-y-5">
            {/* Search */}
            <div>
              <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">{t("common.search")}</label>
              <Input
                placeholder={t("common.search")}
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>

            {/* Status filter */}
            <div>
              <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">{t("common.status")}</label>
              <div className="flex flex-wrap gap-1.5">
                <button
                  onClick={() => setStatusFilter("")}
                  className={`px-3 py-1.5 text-xs rounded-full border transition-all duration-150 font-medium ${
                    !statusFilter
                      ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                      : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)]"
                  }`}
                >
                  All
                </button>
                {RELEASE_STATUSES.map((status) => (
                  <button
                    key={status}
                    onClick={() => setStatusFilter(status === statusFilter ? "" : status)}
                    className={`px-3 py-1.5 text-xs rounded-full border transition-all duration-150 font-medium ${
                      status === statusFilter
                        ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
                        : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)]"
                    }`}
                  >
                    {t(`release.${status}`)}
                  </button>
                ))}
              </div>
            </div>

            {(search || statusFilter) && (
              <button
                onClick={() => { setSearch(""); setStatusFilter(""); }}
                className="w-full py-2 text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors"
              >
                Clear all filters
              </button>
            )}
          </div>
        </Drawer>
      </div>

      {/* Create modal */}
      <ResponsiveModal open={showForm} onClose={() => setShowForm(false)} title={t("release.create")}>
        <ReleaseForm
          projects={projects}
          onSuccess={() => {
            setShowForm(false);
            queryClient.invalidateQueries({ queryKey: ["releases"] });
          }}
        />
      </ResponsiveModal>

      {/* Edit modal */}
      <ResponsiveModal open={!!editRelease} onClose={() => setEditRelease(null)} title={t("release.editRelease")}>
        {editRelease && (
          <ReleaseForm
            release={editRelease}
            projects={projects}
            onSuccess={() => {
              setEditRelease(null);
              queryClient.invalidateQueries({ queryKey: ["releases"] });
            }}
            onDelete={user?.role === "admin" ? () => {
              deleteMutation.mutate(editRelease.id);
              setEditRelease(null);
            } : undefined}
          />
        )}
      </ResponsiveModal>
    </PageShell>
  );
}

function ReleaseForm({ release, projects, onSuccess, onDelete }: {
  release?: Release & { issue_ids?: number[] };
  projects: { id: number; name: string }[];
  onSuccess: () => void;
  onDelete?: () => void;
}) {
  const { t } = useLocale();
  const [step, setStep] = useState(1);
  const [form, setForm] = useState({
    title: release?.title || "",
    description: release?.description || "",
    status: release?.status || "pending",
    project_id: release?.project_id || null as number | null,
    target_date: release?.target_date || "",
    live_date: release?.live_date || "",
    issue_ids: release?.issue_ids || [] as number[],
  });
  const [error, setError] = useState("");

  // Load issues for the selected project
  const { data: projectIssues = [] } = useQuery({
    queryKey: ["project-issues-all", form.project_id],
    queryFn: () => form.project_id ? issuesAPI.listByProject(form.project_id) : Promise.resolve([]),
    enabled: !!form.project_id,
  });

  const mutation = useMutation({
    mutationFn: () => {
      if (release) {
        return releasesAPI.update(release.id, form);
      }
      return releasesAPI.create(form);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed"),
  });

  const set = (key: string, value: unknown) => setForm((f) => ({ ...f, [key]: value }));

  const stepLabels = [t("release.basicInfo") || "Basic Info", t("release.linkedIssues") || "Linked Issues"];

  const handleNext = () => {
    if (!form.title.trim()) {
      setError(t("release.titleRequired") || "Title is required");
      return;
    }
    setError("");
    setStep(2);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (step === 1) {
      handleNext();
      return;
    }
    mutation.mutate();
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <StepIndicator steps={stepLabels} current={step} />
      <FormError message={error} />

      {step === 1 && (
        <>
          <Input label={t("release.titleField")} value={form.title} onChange={(e) => set("title", e.target.value)} required autoFocus />
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Select
              label={t("release.project")}
              value={form.project_id?.toString() || ""}
              onChange={(e) => set("project_id", e.target.value ? parseInt(e.target.value) : null)}
              options={[{ value: "", label: "-" }, ...projects.map((p) => ({ value: p.id.toString(), label: p.name }))]}
            />
            <Select
              label={t("common.status")}
              value={form.status}
              onChange={(e) => set("status", e.target.value)}
              options={RELEASE_STATUSES.map((s) => ({ value: s, label: t(`release.${s}`) }))}
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <Input label={t("release.targetDate")} type="date" value={form.target_date} onChange={(e) => set("target_date", e.target.value)} />
            <Input label={t("release.liveDate")} type="date" value={form.live_date} onChange={(e) => set("live_date", e.target.value)} />
          </div>
          <MarkdownEditor label={t("common.description")} value={form.description} onChange={(v) => set("description", v)} rows={4} />
        </>
      )}

      {step === 2 && (
        <>
          {form.project_id ? (
            <CheckboxList
              label={t("release.linkedIssues")}
              items={projectIssues.map((i) => ({ id: i.id, name: i.title }))}
              selected={form.issue_ids}
              onChange={(ids) => set("issue_ids", ids)}
            />
          ) : (
            <p className="text-sm text-[var(--text-muted)] py-4 text-center">
              {t("release.selectProjectFirst") || "Select a project in Step 1 to link issues."}
            </p>
          )}
        </>
      )}

      <div className="flex items-center justify-between pt-2">
        {step === 1 ? (
          onDelete ? (
            <button
              type="button"
              onClick={() => { if (confirm("Delete this release?")) onDelete(); }}
              className="text-xs text-[var(--text-faint)] hover:text-red-400 transition-colors"
            >
              {t("common.delete")}
            </button>
          ) : <div />
        ) : (
          <Button type="button" variant="secondary" size="sm" onClick={() => setStep(1)}>
            {t("common.back") || "Back"}
          </Button>
        )}
        <div className="flex items-center gap-2">
          {step === 1 ? (
            <Button type="button" onClick={handleNext}>
              {t("common.next") || "Next"}
            </Button>
          ) : (
            <Button type="submit" loading={mutation.isPending}>
              {release ? t("common.save") : t("common.create")}
            </Button>
          )}
        </div>
      </div>
    </form>
  );
}
