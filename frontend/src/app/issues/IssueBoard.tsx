"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { globalIssuesAPI, usersAPI, hostsAPI, dnsAPI, servicesAPI, projectsAPI } from "@/lib/api";
import { useAuth } from "@/contexts/AuthContext";
import PageShell from "@/components/layout/PageShell";
import Badge from "@/components/ui/Badge";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import MarkdownEditor from "@/components/ui/MarkdownEditor";
import Drawer from "@/components/ui/Drawer";
import CheckboxList from "@/components/ui/CheckboxList";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormError from "@/components/ui/FormError";
import Card from "@/components/ui/Card";
import EmptyState from "@/components/ui/EmptyState";
import { SkeletonCard } from "@/components/ui/Skeleton";
import type { Issue } from "@/lib/types";

// ─── Constants ───────────────────────────────────────────────────────────────

const STATUSES = ["backlog", "todo", "in_progress", "review", "done"] as const;

const statusLabels: Record<string, string> = {
  backlog: "Backlog",
  todo: "To Do",
  in_progress: "In Progress",
  review: "Review",
  done: "Done",
};

const priorityColors: Record<string, string> = {
  critical: "bg-red-400",
  high: "bg-amber-400",
  medium: "bg-cyan-400",
  low: "bg-[var(--text-faint)]",
};

const entityColors: Record<string, string> = {
  host: "cyan",
  dns: "emerald",
  service: "purple",
  project: "amber",
};

// ─── Filters type ────────────────────────────────────────────────────────────

interface Filters {
  entity_type: string;
  status: string;
  priority: string;
  source: string;
}

const emptyFilters: Filters = {
  entity_type: "",
  status: "",
  priority: "",
  source: "",
};

// ─── IssueBoard ──────────────────────────────────────────────────────────────

export default function IssueBoard() {
  const { user } = useAuth();
  const queryClient = useQueryClient();

  const [search, setSearch] = useState("");
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [pendingFilters, setPendingFilters] = useState<Filters>(emptyFilters);
  const [showFilters, setShowFilters] = useState(false);
  const [viewMode, setViewMode] = useState<"kanban" | "list">("kanban");
  const [showCreate, setShowCreate] = useState(false);
  const [editIssue, setEditIssue] = useState<Issue | null>(null);
  const [isMobile, setIsMobile] = useState(false);

  // Sort state for list view
  const [sortField, setSortField] = useState<string>("created_at");
  const [sortDir, setSortDir] = useState<"asc" | "desc">("desc");

  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 768);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  // ── Data queries ────────────────────────────────────────────────────────────

  const { data: issues = [], isLoading } = useQuery({
    queryKey: ["issues", search, filters],
    queryFn: () => {
      const params: Record<string, string> = {};
      if (search) params.search = search;
      Object.entries(filters).forEach(([k, v]) => { if (v) params[k] = v; });
      return globalIssuesAPI.list(params);
    },
  });

  const { data: users = [] } = useQuery({ queryKey: ["users"], queryFn: usersAPI.list });
  const { data: allHosts = [] } = useQuery({ queryKey: ["hosts"], queryFn: () => hostsAPI.list() });
  const { data: allDns = [] } = useQuery({ queryKey: ["dns"], queryFn: dnsAPI.list });
  const { data: allServices = [] } = useQuery({ queryKey: ["services"], queryFn: servicesAPI.list });
  const { data: allProjects = [] } = useQuery({ queryKey: ["projects"], queryFn: projectsAPI.list });

  const canEdit = user?.role === "admin" || user?.role === "editor";
  const activeFilterCount = Object.values(filters).filter(Boolean).length;

  // ── Entity label helper ─────────────────────────────────────────────────────

  const getEntityLabel = useCallback((issue: Issue) => {
    switch (issue.entity_type) {
      case "host": return allHosts.find(h => h.id === issue.entity_id)?.nickname || `Host #${issue.entity_id}`;
      case "dns": return allDns.find(d => d.id === issue.entity_id)?.domain || `DNS #${issue.entity_id}`;
      case "service": return allServices.find(s => s.id === issue.entity_id)?.nickname || `Service #${issue.entity_id}`;
      case "project": return allProjects.find(p => p.id === issue.entity_id)?.name || `Project #${issue.entity_id}`;
      default: return issue.entity_type;
    }
  }, [allHosts, allDns, allServices, allProjects]);

  // ── KPI counts ──────────────────────────────────────────────────────────────

  const kpis = useMemo(() => {
    const totalOpen = issues.filter(i => i.status !== "done").length;
    const critical = issues.filter(i => i.priority === "critical").length;
    const assignedToMe = user
      ? issues.filter(i => Array.isArray(i.assignee_ids) && i.assignee_ids.includes(user.id)).length
      : 0;
    const unassigned = issues.filter(i => !i.assignee_ids || i.assignee_ids.length === 0).length;
    return { totalOpen, critical, assignedToMe, unassigned };
  }, [issues, user]);

  // ── Sorted list ─────────────────────────────────────────────────────────────

  const sortedIssues = useMemo(() => {
    const arr = [...issues];
    arr.sort((a, b) => {
      let va: string | number = "";
      let vb: string | number = "";
      switch (sortField) {
        case "priority": {
          const order = { critical: 0, high: 1, medium: 2, low: 3 };
          va = order[a.priority as keyof typeof order] ?? 4;
          vb = order[b.priority as keyof typeof order] ?? 4;
          break;
        }
        case "status": {
          const sOrder: Record<string, number> = { backlog: 0, todo: 1, in_progress: 2, review: 3, done: 4 };
          va = sOrder[a.status] ?? 5;
          vb = sOrder[b.status] ?? 5;
          break;
        }
        case "title": va = a.title.toLowerCase(); vb = b.title.toLowerCase(); break;
        default: va = a.created_at; vb = b.created_at;
      }
      if (va < vb) return sortDir === "asc" ? -1 : 1;
      if (va > vb) return sortDir === "asc" ? 1 : -1;
      return 0;
    });
    return arr;
  }, [issues, sortField, sortDir]);

  const toggleSort = (field: string) => {
    if (sortField === field) {
      setSortDir(d => d === "asc" ? "desc" : "asc");
    } else {
      setSortField(field);
      setSortDir("asc");
    }
  };

  // ── Filter drawer helpers ────────────────────────────────────────────────────

  const openFilterDrawer = () => {
    setPendingFilters(filters);
    setShowFilters(true);
  };

  const applyFilters = () => {
    setFilters(pendingFilters);
    setShowFilters(false);
  };

  const clearFilters = () => {
    setPendingFilters(emptyFilters);
    setFilters(emptyFilters);
    setShowFilters(false);
  };

  // ── Move mutation ────────────────────────────────────────────────────────────

  const moveMutation = useMutation({
    mutationFn: ({ id, status, position }: { id: number; status: string; position: number }) =>
      globalIssuesAPI.move(id, status, position),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["issues"] });
      queryClient.invalidateQueries({ queryKey: ["hosts"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => globalIssuesAPI.delete(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["issues"] }),
  });

  const issuesByStatus = (status: string) =>
    issues.filter(i => i.status === status).sort((a, b) => a.position - b.position);

  const handleStatusChange = (issue: Issue, newStatus: string) => {
    const columnIssues = issuesByStatus(newStatus);
    const newPosition = columnIssues.length > 0
      ? columnIssues[columnIssues.length - 1].position + 1
      : 0;
    moveMutation.mutate({ id: issue.id, status: newStatus, position: newPosition });
  };

  // ── Assignee initials ────────────────────────────────────────────────────────

  const getInitials = (name: string) => {
    const parts = name.trim().split(/\s+/);
    if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
    return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
  };

  const getAssigneeNames = (issue: Issue): string[] => {
    if (!issue.assignee_ids || issue.assignee_ids.length === 0) return [];
    return issue.assignee_ids
      .map(id => users.find(u => u.id === id)?.display_name || `#${id}`)
      .filter(Boolean);
  };

  // ─────────────────────────────────────────────────────────────────────────────

  return (
    <PageShell>
      {/* ── Header ── */}
      <div className="flex items-center justify-between gap-2 mb-6">
        <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>Issues</h1>
        {canEdit && (
          <div className="hidden sm:block">
            <Button size="sm" onClick={() => setShowCreate(true)}>
              <span className="mr-1">+</span> Add Issue
            </Button>
          </div>
        )}
      </div>

      {/* ── KPI row ── */}
      {!isLoading && (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-6">
          {[
            {
              label: "Total Open",
              value: kpis.totalOpen,
              textColor: "text-cyan-400",
              borderColor: "border-cyan-500/20",
              gradient: "from-cyan-500/10 to-transparent",
              icon: "M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2",
            },
            {
              label: "Critical",
              value: kpis.critical,
              textColor: "text-red-400",
              borderColor: "border-red-500/20",
              gradient: "from-red-500/10 to-transparent",
              icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z",
            },
            {
              label: "Assigned to Me",
              value: kpis.assignedToMe,
              textColor: "text-purple-400",
              borderColor: "border-purple-500/20",
              gradient: "from-purple-500/10 to-transparent",
              icon: "M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z",
            },
            {
              label: "Unassigned",
              value: kpis.unassigned,
              textColor: "text-amber-400",
              borderColor: "border-amber-500/20",
              gradient: "from-amber-500/10 to-transparent",
              icon: "M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z",
            },
          ].map((kpi) => (
            <div
              key={kpi.label}
              className={`relative overflow-hidden bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border ${kpi.borderColor} p-4 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-[var(--shadow-md)]`}
            >
              <div className={`absolute inset-0 bg-gradient-to-br ${kpi.gradient} pointer-events-none`} />
              <svg
                className={`absolute right-2.5 top-2.5 w-8 h-8 ${kpi.textColor} opacity-[0.08]`}
                fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1}
              >
                <path strokeLinecap="round" strokeLinejoin="round" d={kpi.icon} />
              </svg>
              <div className="relative">
                <p className="text-[10px] text-[var(--text-muted)] uppercase tracking-wider font-medium">{kpi.label}</p>
                <p className={`text-2xl font-bold mt-0.5 ${kpi.textColor}`} style={{ fontFamily: "var(--font-display)" }}>{kpi.value}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* ── Toolbar ── */}
      <div className="flex items-center gap-2 mb-5">
        {/* Search */}
        <div className="relative flex-1 max-w-sm min-w-[200px]">
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-faint)] pointer-events-none"
            fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            placeholder="Search issues..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] pl-9 pr-3 py-1.5 text-sm transition-all duration-200 focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)] focus:outline-none placeholder:text-[var(--text-faint)]"
          />
        </div>

        {/* Filter button */}
        <button
          onClick={openFilterDrawer}
          className={`relative flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-[var(--radius-md)] border transition-all ${
            activeFilterCount > 0
              ? "bg-[var(--accent-muted)] text-[var(--accent)] border-[var(--accent)]/20"
              : "bg-[var(--bg-elevated)] text-[var(--text-muted)] border-[var(--border-default)] hover:text-[var(--text-secondary)]"
          }`}
        >
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
          </svg>
          <span className="hidden sm:inline">Filters</span>
          {activeFilterCount > 0 && (
            <span className="absolute -top-1.5 -right-1.5 w-4 h-4 rounded-full bg-[var(--accent)] text-[var(--bg-base)] text-[10px] font-bold flex items-center justify-center">
              {activeFilterCount}
            </span>
          )}
        </button>

        {/* View toggle */}
        <div className="hidden sm:flex border border-[var(--border-default)] rounded-[var(--radius-md)] overflow-hidden">
          <button
            onClick={() => setViewMode("kanban")}
            className={`px-2.5 py-1.5 transition-colors ${viewMode === "kanban" ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"}`}
            title="Kanban view"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2" />
            </svg>
          </button>
          <button
            onClick={() => setViewMode("list")}
            className={`px-2.5 py-1.5 transition-colors ${viewMode === "list" ? "bg-[var(--accent-muted)] text-[var(--accent)]" : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"}`}
            title="List view"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M4 6h16M4 10h16M4 14h16M4 18h16" />
            </svg>
          </button>
        </div>

        {/* Spacer */}
        <div className="flex-1" />

        {/* Desktop add button (duplicate for toolbar) */}
        {canEdit && (
          <div className="hidden sm:block">
            <Button size="sm" variant="secondary" onClick={() => setShowCreate(true)}>
              <span className="mr-1">+</span> Add Issue
            </Button>
          </div>
        )}
        {/* Mobile add */}
        {canEdit && (
          <button
            onClick={() => setShowCreate(true)}
            className="sm:hidden flex items-center justify-center w-8 h-8 rounded-[var(--radius-md)] bg-[var(--accent)] text-[var(--bg-base)] hover:opacity-90 transition-opacity"
            title="Add Issue"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
            </svg>
          </button>
        )}
      </div>

      {/* ── Content ── */}
      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => <SkeletonCard key={i} />)}
        </div>
      ) : issues.length === 0 ? (
        <EmptyState
          icon="folder"
          title="No issues found"
          description={
            search || activeFilterCount
              ? "Try adjusting your search or filters"
              : "Create your first issue to start tracking work"
          }
          action={canEdit && !search && !activeFilterCount ? (
            <Button size="sm" onClick={() => setShowCreate(true)}>+ Add Issue</Button>
          ) : undefined}
        />
      ) : viewMode === "kanban" || isMobile ? (
        <KanbanView
          issues={issues}
          issuesByStatus={issuesByStatus}
          getEntityLabel={getEntityLabel}
          getAssigneeNames={getAssigneeNames}
          getInitials={getInitials}
          handleStatusChange={handleStatusChange}
          onEdit={setEditIssue}
          canEdit={canEdit}
        />
      ) : (
        <ListView
          issues={sortedIssues}
          getEntityLabel={getEntityLabel}
          getAssigneeNames={getAssigneeNames}
          sortField={sortField}
          sortDir={sortDir}
          onSort={toggleSort}
          onEdit={setEditIssue}
        />
      )}

      {/* ── Filter drawer ── */}
      <Drawer
        open={showFilters}
        onClose={() => setShowFilters(false)}
        title="Filters"
        footer={
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" className="flex-1" onClick={clearFilters}>Clear</Button>
            <Button size="sm" className="flex-1" onClick={applyFilters}>Apply</Button>
          </div>
        }
      >
        <div className="space-y-4">
          <Select
            label="Entity type"
            value={pendingFilters.entity_type}
            onChange={(e) => setPendingFilters(f => ({ ...f, entity_type: e.target.value }))}
            options={[
              { value: "", label: "Any" },
              { value: "host", label: "Host" },
              { value: "dns", label: "DNS" },
              { value: "service", label: "Service" },
              { value: "project", label: "Project" },
            ]}
          />
          <Select
            label="Status"
            value={pendingFilters.status}
            onChange={(e) => setPendingFilters(f => ({ ...f, status: e.target.value }))}
            options={[
              { value: "", label: "Any" },
              { value: "backlog", label: "Backlog" },
              { value: "todo", label: "To Do" },
              { value: "in_progress", label: "In Progress" },
              { value: "review", label: "Review" },
              { value: "done", label: "Done" },
            ]}
          />
          <Select
            label="Priority"
            value={pendingFilters.priority}
            onChange={(e) => setPendingFilters(f => ({ ...f, priority: e.target.value }))}
            options={[
              { value: "", label: "Any" },
              { value: "low", label: "Low" },
              { value: "medium", label: "Medium" },
              { value: "high", label: "High" },
              { value: "critical", label: "Critical" },
            ]}
          />
          <Select
            label="Source"
            value={pendingFilters.source}
            onChange={(e) => setPendingFilters(f => ({ ...f, source: e.target.value }))}
            options={[
              { value: "", label: "Any" },
              { value: "manual", label: "Manual" },
              { value: "alert", label: "Alert" },
            ]}
          />
        </div>
      </Drawer>

      {/* ── Create modal ── */}
      <ResponsiveModal open={showCreate} onClose={() => setShowCreate(false)} title="New Issue">
        <IssueForm
          users={users}
          allHosts={allHosts}
          allDns={allDns}
          allServices={allServices}
          allProjects={allProjects}
          onSuccess={() => {
            setShowCreate(false);
            queryClient.invalidateQueries({ queryKey: ["issues"] });
          }}
        />
      </ResponsiveModal>

      {/* ── Edit modal ── */}
      <ResponsiveModal
        open={!!editIssue}
        onClose={() => setEditIssue(null)}
        title="Edit Issue"
      >
        {editIssue && (
          <IssueForm
            issue={editIssue}
            users={users}
            allHosts={allHosts}
            allDns={allDns}
            allServices={allServices}
            allProjects={allProjects}
            onSuccess={() => {
              setEditIssue(null);
              queryClient.invalidateQueries({ queryKey: ["issues"] });
              queryClient.invalidateQueries({ queryKey: ["hosts"] });
            }}
            onDelete={user?.role === "admin" ? () => {
              deleteMutation.mutate(editIssue.id);
              setEditIssue(null);
            } : undefined}
          />
        )}
      </ResponsiveModal>
    </PageShell>
  );
}

// ─── KanbanView ───────────────────────────────────────────────────────────────

interface KanbanViewProps {
  issues: Issue[];
  issuesByStatus: (status: string) => Issue[];
  getEntityLabel: (issue: Issue) => string;
  getAssigneeNames: (issue: Issue) => string[];
  getInitials: (name: string) => string;
  handleStatusChange: (issue: Issue, status: string) => void;
  onEdit: (issue: Issue) => void;
  canEdit: boolean;
}

function KanbanView({
  issues,
  issuesByStatus,
  getEntityLabel,
  getAssigneeNames,
  getInitials,
  handleStatusChange,
  onEdit,
  canEdit,
}: KanbanViewProps) {
  return (
    <div className="grid grid-cols-1 sm:grid-cols-3 lg:grid-cols-5 gap-3" style={{ minHeight: "420px" }}>
      {STATUSES.map((status) => {
        const columnIssues = issuesByStatus(status);
        return (
          <div
            key={status}
            className="flex flex-col rounded-[var(--radius-lg)] bg-[var(--bg-surface)] border border-[var(--border-subtle)] overflow-hidden"
          >
            {/* Column header */}
            <div className="px-3 py-2.5 border-b border-[var(--border-subtle)] flex items-center justify-between">
              <span
                className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]"
                style={{ fontFamily: "var(--font-display)" }}
              >
                {statusLabels[status]}
              </span>
              <span className="text-[10px] font-medium text-[var(--text-faint)] bg-[var(--bg-elevated)] rounded-full px-1.5 py-0.5">
                {columnIssues.length}
              </span>
            </div>

            {/* Cards */}
            <div className="flex-1 p-2 space-y-2 overflow-y-auto">
              {columnIssues.length === 0 && (
                <div className="text-center py-6 text-xs text-[var(--text-faint)]">No issues</div>
              )}
              {columnIssues.map((issue) => (
                <IssueCard
                  key={issue.id}
                  issue={issue}
                  entityLabel={getEntityLabel(issue)}
                  assigneeNames={getAssigneeNames(issue)}
                  getInitials={getInitials}
                  onEdit={() => onEdit(issue)}
                  onStatusChange={canEdit ? (s) => handleStatusChange(issue, s) : undefined}
                  canEdit={canEdit}
                />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ─── IssueCard ────────────────────────────────────────────────────────────────

interface IssueCardProps {
  issue: Issue;
  entityLabel: string;
  assigneeNames: string[];
  getInitials: (name: string) => string;
  onEdit: () => void;
  onStatusChange?: (status: string) => void;
  canEdit: boolean;
}

function IssueCard({
  issue,
  entityLabel,
  assigneeNames,
  getInitials,
  onEdit,
  onStatusChange,
  canEdit,
}: IssueCardProps) {
  const [showStatusMenu, setShowStatusMenu] = useState(false);

  return (
    <Card onClick={canEdit ? onEdit : undefined} hover={canEdit} clickIndicator={canEdit ? "drawer" : undefined} className="!p-3">
      {/* Title row */}
      <div className="flex items-start gap-1.5 mb-2">
        <span className={`w-2 h-2 rounded-full shrink-0 mt-1.5 ${priorityColors[issue.priority] || "bg-[var(--text-faint)]"}`} />
        <div className="min-w-0 flex-1">
          <span className="text-sm font-medium text-[var(--text-primary)] line-clamp-2 leading-tight block">
            {issue.title}
          </span>
          <div className="flex items-center gap-1.5 mt-1 flex-wrap">
            {issue.entity_type && (
              <span
                className={`text-[10px] rounded px-1.5 py-0.5 ${
                  entityColors[issue.entity_type]
                    ? `text-${entityColors[issue.entity_type]}-400 bg-${entityColors[issue.entity_type]}-500/10`
                    : "text-[var(--text-faint)] bg-[var(--bg-overlay)]"
                }`}
              >
                {issue.entity_type}: {entityLabel}
              </span>
            )}
            {issue.source === "alert" && (
              <span className="text-[10px] text-red-400 bg-red-500/10 rounded px-1.5 py-0.5 font-medium">alert</span>
            )}
          </div>
        </div>
      </div>

      {/* Meta row */}
      <div className="flex items-center gap-1.5 flex-wrap text-xs">
        {issue.expected_end_date && (
          <span className="text-[var(--text-faint)] ml-auto" style={{ fontFamily: "var(--font-mono)" }}>
            {issue.expected_end_date}
          </span>
        )}
        {assigneeNames.length > 0 && (
          <div className={`flex -space-x-1 ${!issue.expected_end_date ? "ml-auto" : ""}`}>
            {assigneeNames.slice(0, 3).map((name) => (
              <span
                key={name}
                title={name}
                className="w-5 h-5 rounded-full bg-[var(--accent-muted)] text-[var(--accent)] text-[8px] font-bold flex items-center justify-center border border-[var(--bg-surface)]"
              >
                {getInitials(name)}
              </span>
            ))}
            {assigneeNames.length > 3 && (
              <span className="w-5 h-5 rounded-full bg-[var(--bg-overlay)] text-[var(--text-faint)] text-[8px] font-bold flex items-center justify-center border border-[var(--bg-surface)]">
                +{assigneeNames.length - 3}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Quick status move (stop propagation to prevent edit modal opening) */}
      {canEdit && onStatusChange && (
        <div
          className="relative mt-1.5"
          onClick={(e) => e.stopPropagation()}
        >
          <button
            onClick={() => setShowStatusMenu(v => !v)}
            className="text-[10px] text-[var(--text-faint)] hover:text-[var(--text-muted)] transition-colors flex items-center gap-0.5"
          >
            <svg className="w-2.5 h-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l4-4 4 4m0 6l-4 4-4-4" />
            </svg>
            Move
          </button>
          {showStatusMenu && (
            <div className="absolute bottom-full left-0 mb-1 z-20 bg-[var(--bg-surface)] border border-[var(--border-default)] rounded-[var(--radius-md)] shadow-lg py-1 min-w-[120px]">
              {STATUSES.filter(s => s !== issue.status).map(s => (
                <button
                  key={s}
                  onClick={() => { onStatusChange(s); setShowStatusMenu(false); }}
                  className="w-full text-left px-3 py-1.5 text-xs text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] transition-colors"
                >
                  {statusLabels[s]}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </Card>
  );
}

// ─── ListView ─────────────────────────────────────────────────────────────────

interface ListViewProps {
  issues: Issue[];
  getEntityLabel: (issue: Issue) => string;
  getAssigneeNames: (issue: Issue) => string[];
  sortField: string;
  sortDir: "asc" | "desc";
  onSort: (field: string) => void;
  onEdit: (issue: Issue) => void;
}

function ListView({ issues, getEntityLabel, getAssigneeNames, sortField, sortDir, onSort, onEdit }: ListViewProps) {
  const SortIcon = ({ field }: { field: string }) => (
    <svg
      className={`w-3 h-3 ml-1 inline-block transition-colors ${sortField === field ? "text-[var(--accent)]" : "text-[var(--text-faint)]"}`}
      fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
    >
      {sortField === field && sortDir === "asc"
        ? <path strokeLinecap="round" strokeLinejoin="round" d="M5 15l7-7 7 7" />
        : <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
      }
    </svg>
  );

  return (
    <div className="border border-[var(--border-default)] rounded-[var(--radius-lg)] overflow-x-auto animate-fade-in">
      <table className="w-full text-sm min-w-[640px]">
        <thead>
          <tr className="bg-[var(--bg-elevated)] text-[var(--text-muted)] text-[11px] uppercase tracking-wider">
            <th
              className="text-left px-4 py-3 font-semibold cursor-pointer select-none hover:text-[var(--text-secondary)] w-8"
              onClick={() => onSort("priority")}
            >
              <span className="flex items-center">P<SortIcon field="priority" /></span>
            </th>
            <th
              className="text-left px-4 py-3 font-semibold cursor-pointer select-none hover:text-[var(--text-secondary)]"
              onClick={() => onSort("title")}
            >
              <span className="flex items-center">Title<SortIcon field="title" /></span>
            </th>
            <th className="text-left px-4 py-3 font-semibold">Entity</th>
            <th
              className="text-left px-4 py-3 font-semibold cursor-pointer select-none hover:text-[var(--text-secondary)]"
              onClick={() => onSort("status")}
            >
              <span className="flex items-center">Status<SortIcon field="status" /></span>
            </th>
            <th className="text-left px-4 py-3 font-semibold">Assignees</th>
            <th
              className="text-left px-4 py-3 font-semibold cursor-pointer select-none hover:text-[var(--text-secondary)] hidden sm:table-cell"
              onClick={() => onSort("created_at")}
            >
              <span className="flex items-center">Created<SortIcon field="created_at" /></span>
            </th>
          </tr>
        </thead>
        <tbody>
          {issues.map((issue, i) => {
            const assignees = getAssigneeNames(issue);
            return (
              <tr
                key={issue.id}
                onClick={() => onEdit(issue)}
                className={`border-t border-[var(--border-subtle)] hover:bg-[var(--bg-elevated)] transition-colors cursor-pointer ${i % 2 === 1 ? "bg-[var(--bg-surface)]" : ""}`}
              >
                {/* Priority dot */}
                <td className="px-4 py-2.5">
                  <span
                    title={issue.priority}
                    className={`block w-2 h-2 rounded-full ${priorityColors[issue.priority] || "bg-[var(--text-faint)]"}`}
                  />
                </td>

                {/* Title */}
                <td className="px-4 py-2.5 font-medium text-[var(--text-primary)] max-w-xs">
                  <span className="line-clamp-1">{issue.title}</span>
                  {issue.source === "alert" && (
                    <span className="ml-1.5 text-[10px] text-red-400 bg-red-500/10 rounded px-1.5 py-0.5 font-medium">alert</span>
                  )}
                </td>

                {/* Entity */}
                <td className="px-4 py-2.5">
                  {issue.entity_type ? (
                    <Badge color={entityColors[issue.entity_type] as "cyan" | "emerald" | "purple" | "amber" | undefined}>
                      {issue.entity_type}: {getEntityLabel(issue)}
                    </Badge>
                  ) : (
                    <span className="text-[var(--text-faint)] text-xs">—</span>
                  )}
                </td>

                {/* Status */}
                <td className="px-4 py-2.5">
                  <span
                    className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border ${
                      issue.status === "done"
                        ? "bg-emerald-500/15 text-emerald-400 border-emerald-500/30"
                        : issue.status === "in_progress"
                        ? "bg-cyan-500/15 text-cyan-400 border-cyan-500/30"
                        : issue.status === "review"
                        ? "bg-purple-500/15 text-purple-400 border-purple-500/30"
                        : "bg-[var(--bg-overlay)] text-[var(--text-muted)] border-[var(--border-default)]"
                    }`}
                  >
                    {statusLabels[issue.status] || issue.status}
                  </span>
                </td>

                {/* Assignees */}
                <td className="px-4 py-2.5">
                  {assignees.length > 0 ? (
                    <div className="flex -space-x-1">
                      {assignees.slice(0, 4).map((name) => (
                        <span
                          key={name}
                          title={name}
                          className="w-6 h-6 rounded-full bg-[var(--accent-muted)] text-[var(--accent)] text-[10px] font-bold flex items-center justify-center border-2 border-[var(--bg-base)]"
                        >
                          {name.slice(0, 2).toUpperCase()}
                        </span>
                      ))}
                      {assignees.length > 4 && (
                        <span className="w-6 h-6 rounded-full bg-[var(--bg-overlay)] text-[var(--text-faint)] text-[10px] font-bold flex items-center justify-center border-2 border-[var(--bg-base)]">
                          +{assignees.length - 4}
                        </span>
                      )}
                    </div>
                  ) : (
                    <span className="text-xs text-[var(--text-faint)]">—</span>
                  )}
                </td>

                {/* Created */}
                <td className="px-4 py-2.5 text-xs text-[var(--text-muted)] hidden sm:table-cell">
                  {new Date(issue.created_at).toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" })}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

// ─── IssueForm ────────────────────────────────────────────────────────────────

interface IssueFormProps {
  issue?: Issue;
  users: import("@/lib/types").User[];
  allHosts: import("@/lib/types").Host[];
  allDns: import("@/lib/types").DNSRecord[];
  allServices: import("@/lib/types").Service[];
  allProjects: import("@/lib/types").Project[];
  onSuccess: () => void;
  onDelete?: () => void;
}

function IssueForm({
  issue,
  users,
  allHosts,
  allDns,
  allServices,
  allProjects,
  onSuccess,
  onDelete,
}: IssueFormProps) {
  const [form, setForm] = useState({
    title: issue?.title || "",
    description: issue?.description || "",
    entity_type: issue?.entity_type || "",
    entity_id: issue?.entity_id ? issue.entity_id.toString() : "",
    status: issue?.status || "backlog",
    priority: issue?.priority || "medium",
    assignee_ids: issue?.assignee_ids || ([] as number[]),
    expected_end_date: issue?.expected_end_date || "",
  });
  const [error, setError] = useState("");

  const set = (k: string, v: unknown) => setForm(f => ({ ...f, [k]: v }));

  // Reset entity_id when entity_type changes
  const handleEntityTypeChange = (v: string) => {
    setForm(f => ({ ...f, entity_type: v, entity_id: "" }));
  };

  // Dynamic entity options based on selected type
  const entityOptions = useMemo(() => {
    switch (form.entity_type) {
      case "host": return allHosts.map(h => ({ value: h.id.toString(), label: h.nickname }));
      case "dns": return allDns.map(d => ({ value: d.id.toString(), label: d.domain }));
      case "service": return allServices.map(s => ({ value: s.id.toString(), label: s.nickname }));
      case "project": return allProjects.map(p => ({ value: p.id.toString(), label: p.name }));
      default: return [];
    }
  }, [form.entity_type, allHosts, allDns, allServices, allProjects]);

  const userItems = useMemo(
    () => users.map(u => ({ id: u.id, name: u.display_name })),
    [users]
  );

  const mutation = useMutation({
    mutationFn: () => {
      const payload = {
        ...form,
        entity_id: form.entity_id ? parseInt(form.entity_id, 10) : undefined,
      };
      if (issue) {
        return globalIssuesAPI.update(issue.id, payload);
      }
      return globalIssuesAPI.create(payload);
    },
    onSuccess: () => onSuccess(),
    onError: (err) => setError(err instanceof Error ? err.message : "Failed to save issue"),
  });

  return (
    <form
      className="space-y-4"
      onSubmit={(e) => {
        e.preventDefault();
        setError("");
        mutation.mutate();
      }}
    >
      <Input
        label="Title"
        value={form.title}
        onChange={(e) => set("title", e.target.value)}
        placeholder="Issue title..."
        required
      />

      <MarkdownEditor
        label="Description"
        value={form.description}
        onChange={(v) => set("description", v)}
        placeholder="Describe the issue..."
        rows={3}
      />

      <div className="grid grid-cols-2 gap-3">
        <Select
          label="Entity type"
          value={form.entity_type}
          onChange={(e) => handleEntityTypeChange(e.target.value)}
          options={[
            { value: "", label: "None" },
            { value: "host", label: "Host" },
            { value: "dns", label: "DNS" },
            { value: "service", label: "Service" },
            { value: "project", label: "Project" },
          ]}
        />
        <Select
          label="Entity"
          value={form.entity_id}
          onChange={(e) => set("entity_id", e.target.value)}
          options={[{ value: "", label: "Select..." }, ...entityOptions]}
          disabled={!form.entity_type || entityOptions.length === 0}
        />
      </div>

      <div className="grid grid-cols-2 gap-3">
        <Select
          label="Status"
          value={form.status}
          onChange={(e) => set("status", e.target.value)}
          options={[
            { value: "backlog", label: "Backlog" },
            { value: "todo", label: "To Do" },
            { value: "in_progress", label: "In Progress" },
            { value: "review", label: "Review" },
            { value: "done", label: "Done" },
          ]}
        />
        <Select
          label="Priority"
          value={form.priority}
          onChange={(e) => set("priority", e.target.value)}
          options={[
            { value: "low", label: "Low" },
            { value: "medium", label: "Medium" },
            { value: "high", label: "High" },
            { value: "critical", label: "Critical" },
          ]}
        />
      </div>

      <Input
        label="Expected End Date"
        type="date"
        value={form.expected_end_date}
        onChange={(e) => set("expected_end_date", e.target.value)}
      />

      {userItems.length > 0 && (
        <CheckboxList
          label="Assignees"
          items={userItems}
          selected={form.assignee_ids}
          onChange={(ids) => set("assignee_ids", ids)}
        />
      )}

      {/* Read-only source info for alert-sourced issues */}
      {issue && issue.source && issue.source !== "manual" && (
        <div className="bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-md)] p-3 space-y-1.5">
          <p className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wide">Source</p>
          <div className="flex items-center gap-2">
            <span className="text-xs text-red-400 bg-red-500/10 rounded px-1.5 py-0.5 font-medium">{issue.source}</span>
            {issue.source_ref && (
              <span className="text-xs text-[var(--text-faint)] font-mono">{issue.source_ref}</span>
            )}
          </div>
        </div>
      )}

      <FormError message={error} />

      <div className="flex gap-2 pt-1">
        <Button type="submit" className="flex-1" disabled={mutation.isPending}>
          {mutation.isPending ? "Saving..." : issue ? "Save changes" : "Create issue"}
        </Button>
        {onDelete && (
          <Button
            type="button"
            variant="secondary"
            onClick={onDelete}
            className="text-red-400 border-red-500/20 hover:bg-red-500/10"
          >
            Delete
          </Button>
        )}
      </div>
    </form>
  );
}
