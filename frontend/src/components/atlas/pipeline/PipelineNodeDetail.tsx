"use client";

import { useLocale } from "@/contexts/LocaleContext";
import type { AtlasIndexes } from "@/lib/atlas/types";
import type { LineageNode } from "@/lib/lineage/types";
import { NODE_TYPE_LABELS } from "@/lib/lineage/style";
import TableDetailPanel from "../catalog/TableDetailPanel";

interface Props {
  indexes: AtlasIndexes;
  nodeId: string;
  onSelectNode: (id: string) => void;
  onSelectColumn: (columnId: string) => void;
}

export default function PipelineNodeDetail({ indexes, nodeId, onSelectNode, onSelectColumn }: Props) {
  const node = indexes.nodesById.get(nodeId);
  if (!node) return null;

  // Tables reuse the catalog's table panel for consistency.
  if (node.type === "table") {
    return (
      <TableDetailPanel
        indexes={indexes}
        tableId={nodeId}
        onSelectTable={onSelectNode}
        onSelectColumn={onSelectColumn}
      />
    );
  }

  if (node.type === "dag") return <DagPanel node={node} indexes={indexes} onSelectNode={onSelectNode} />;
  if (node.type === "task") return <TaskPanel node={node} indexes={indexes} onSelectNode={onSelectNode} />;
  if (node.type === "dbt_model") return <ModelPanel node={node} indexes={indexes} onSelectNode={onSelectNode} />;
  if (node.type === "dbt_source") return <SourcePanel node={node} indexes={indexes} onSelectNode={onSelectNode} />;
  return <GenericPanel node={node} />;
}

// -- DAG ----------------------------------------------------------------------

function DagPanel({ node, indexes, onSelectNode }: { node: LineageNode; indexes: AtlasIndexes; onSelectNode: (id: string) => void }) {
  const { t } = useLocale();
  const d = node.data as Record<string, unknown> | undefined;
  const taskIds = indexes.childrenOf.get(node.id) ?? [];
  const tasks = taskIds.map(id => indexes.nodesById.get(id)).filter((n): n is LineageNode => n?.type === "task");
  const tags = Array.isArray(d?.tags) ? (d!.tags as string[]) : [];

  return (
    <div className="flex flex-col gap-5">
      <SectionHeader type={node.type} label={node.label} namespace={node.namespace} />
      <KeyValueGrid items={[
        ["nav.atlas.pipeline.detail.schedule", t("atlas.pipeline.detail.schedule"), d?.schedule],
        ["owner", t("atlas.pipeline.detail.owner"), d?.owner],
        ["taskCount", t("atlas.pipeline.detail.taskCount"), tasks.length],
        ["tags", t("atlas.pipeline.detail.tags"), tags.join(", ") || "—"],
      ]} />
      <Section label={t("atlas.pipeline.detail.taskCount")}>
        <div className="flex flex-col gap-1">
          {tasks.length === 0 ? <Empty /> : tasks.map(t => (
            <NodeLink key={t.id} node={t} onSelect={() => onSelectNode(t.id)} />
          ))}
        </div>
      </Section>
    </div>
  );
}

// -- Task ---------------------------------------------------------------------

function TaskPanel({ node, indexes, onSelectNode }: { node: LineageNode; indexes: AtlasIndexes; onSelectNode: (id: string) => void }) {
  const { t } = useLocale();
  const d = node.data as Record<string, unknown> | undefined;
  const reads = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "reads").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const writes = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "writes").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const executes = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "executes").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const parent = node.parent ? indexes.nodesById.get(node.parent) : null;

  return (
    <div className="flex flex-col gap-5">
      <SectionHeader type={node.type} label={node.label} namespace={node.namespace} />
      <KeyValueGrid items={[
        ["operator", t("atlas.pipeline.detail.operator"), d?.operator],
        ["pythonCallable", t("atlas.pipeline.detail.pythonCallable"), d?.python_callable],
        ["parent", "DAG", parent?.label],
      ]} />
      {executes.length > 0 && (
        <Section label={t("atlas.pipeline.detail.executes")}>
          {executes.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
      {reads.length > 0 && (
        <Section label={t("atlas.pipeline.detail.reads")}>
          {reads.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
      {writes.length > 0 && (
        <Section label={t("atlas.pipeline.detail.writes")}>
          {writes.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
    </div>
  );
}

// -- dbt Model ----------------------------------------------------------------

function ModelPanel({ node, indexes, onSelectNode }: { node: LineageNode; indexes: AtlasIndexes; onSelectNode: (id: string) => void }) {
  const { t } = useLocale();
  const d = node.data as Record<string, unknown> | undefined;
  const refs = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "ref").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const sources = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "uses_source").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const writes = (indexes.outEdges.get(node.id) ?? []).filter(e => e.kind === "writes").map(e => indexes.nodesById.get(e.target)).filter(Boolean) as LineageNode[];
  const macros = Array.isArray(d?.macros_called) ? (d!.macros_called as string[]) : [];

  return (
    <div className="flex flex-col gap-5">
      <SectionHeader type={node.type} label={node.label} namespace={node.namespace} layer={node.layer} />
      <KeyValueGrid items={[
        ["materialized", t("atlas.pipeline.detail.materialized"), d?.materialized],
        ["schema", t("atlas.pipeline.detail.schema"), d?.schema],
        ["unique_key", "Unique key", d?.unique_key],
      ]} />
      {refs.length > 0 && (
        <Section label={t("atlas.pipeline.detail.refs")}>
          {refs.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
      {sources.length > 0 && (
        <Section label={t("atlas.pipeline.detail.sources")}>
          {sources.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
      {writes.length > 0 && (
        <Section label={t("atlas.pipeline.detail.writes")}>
          {writes.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
      {macros.length > 0 && (
        <Section label={t("atlas.pipeline.detail.macros")}>
          <div className="flex flex-wrap gap-1">
            {macros.map(m => (
              <span key={m} className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-elevated)] text-[var(--text-secondary)] border border-[var(--border-subtle)]">{m}</span>
            ))}
          </div>
        </Section>
      )}
    </div>
  );
}

// -- dbt Source ---------------------------------------------------------------

function SourcePanel({ node, indexes, onSelectNode }: { node: LineageNode; indexes: AtlasIndexes; onSelectNode: (id: string) => void }) {
  const d = node.data as Record<string, unknown> | undefined;
  const consumers = (indexes.inEdges.get(node.id) ?? []).filter(e => e.kind === "uses_source").map(e => indexes.nodesById.get(e.source)).filter(Boolean) as LineageNode[];
  return (
    <div className="flex flex-col gap-5">
      <SectionHeader type={node.type} label={node.label} namespace={node.namespace} />
      <KeyValueGrid items={[
        ["catalog", "Catalog", d?.catalog],
        ["schema", "Schema", d?.schema],
      ]} />
      {consumers.length > 0 && (
        <Section label="Used by">
          {consumers.map(n => <NodeLink key={n.id} node={n} onSelect={() => onSelectNode(n.id)} />)}
        </Section>
      )}
    </div>
  );
}

function GenericPanel({ node }: { node: LineageNode }) {
  return (
    <div className="flex flex-col gap-5">
      <SectionHeader type={node.type} label={node.label} namespace={node.namespace} />
      {node.data && (
        <pre className="text-[10px] font-mono p-2 rounded bg-[var(--bg-base)] border border-[var(--border-subtle)] text-[var(--text-secondary)] whitespace-pre-wrap break-all">
          {JSON.stringify(node.data, null, 2)}
        </pre>
      )}
    </div>
  );
}

// -- Shared atoms -------------------------------------------------------------

function SectionHeader({ type, label, namespace, layer }: { type: string; label: string; namespace?: string; layer?: string }) {
  return (
    <header className="flex flex-col gap-1 -mt-1">
      <div className="flex items-center gap-2 flex-wrap">
        <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">
          {NODE_TYPE_LABELS[type] ?? type}
        </span>
        {namespace && <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)]">{namespace}</span>}
        {layer && <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)]">{layer}</span>}
      </div>
      <h2 className="text-lg font-bold text-[var(--text-primary)]" style={{ fontFamily: "var(--font-display)" }}>{label}</h2>
    </header>
  );
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <span className="text-[10px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">{label}</span>
      {children}
    </section>
  );
}

function KeyValueGrid({ items }: { items: Array<[string, string, unknown]> }) {
  return (
    <div className="grid grid-cols-2 gap-2">
      {items.map(([key, label, value]) => (
        <KV key={key} label={label} value={value} />
      ))}
    </div>
  );
}

function KV({ label, value }: { label: string; value: unknown }) {
  const display = value === null || value === undefined || value === "" ? "—" : String(value);
  return (
    <div className="rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-base)] p-2.5 flex flex-col gap-0.5 min-w-0">
      <span className="text-[9px] uppercase tracking-[0.14em] text-[var(--text-faint)] font-semibold">{label}</span>
      <span className="text-[12px] font-mono text-[var(--text-primary)] truncate" title={display}>{display}</span>
    </div>
  );
}

function NodeLink({ node, onSelect }: { node: LineageNode; onSelect: () => void }) {
  return (
    <button
      onClick={onSelect}
      className="w-full text-left flex items-center gap-2 px-2 py-1.5 rounded-[var(--radius-sm)] border border-[var(--border-subtle)] bg-[var(--bg-base)] hover:bg-[var(--bg-elevated)] hover:border-[var(--border-default)] transition-colors"
    >
      <span className="text-[9px] uppercase tracking-wider text-[var(--text-faint)] font-mono shrink-0">{NODE_TYPE_LABELS[node.type] ?? node.type}</span>
      <span className="text-[12px] font-mono text-[var(--text-primary)] truncate">{node.label}</span>
    </button>
  );
}

function Empty() {
  return <p className="text-xs text-[var(--text-muted)] italic">—</p>;
}
