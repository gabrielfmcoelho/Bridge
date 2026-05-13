"use client";

import { useEffect, useMemo, useState } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  type Node,
  type NodeMouseHandler,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { AtlasIndexes, AtlasFilters } from "@/lib/atlas/types";
import { groupNodesForNested } from "@/lib/atlas/groupNodesForNested";
import EntityNode from "../nodes/EntityNode";
import DomainFrameNode from "../nodes/DomainFrameNode";
import DagFrameNode from "../nodes/DagFrameNode";
import { minimapColor } from "@/lib/lineage/style";

const NODE_TYPES = {
  pipelineEntity: EntityNode,
  pipelineDomainFrame: DomainFrameNode,
  pipelineDagFrame: DagFrameNode,
};

interface Props {
  indexes: AtlasIndexes;
  filters: AtlasFilters;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
}

export default function NestedGroupsGraph({ indexes, filters, selectedId, onSelect }: Props) {
  const [collapsedDags, setCollapsedDags] = useState<Set<string>>(new Set());

  const layout = useMemo(
    () => groupNodesForNested(indexes, filters, collapsedDags),
    [indexes, filters, collapsedDags],
  );
  const decorated = useMemo<Node[]>(() => layout.nodes.map(n => (
    n.id === selectedId ? { ...n, selected: true } : n
  )), [layout.nodes, selectedId]);

  const [nodes, setNodes, onNodesChange] = useNodesState(decorated);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layout.edges);

  useEffect(() => setNodes(decorated), [decorated, setNodes]);
  useEffect(() => setEdges(layout.edges), [layout.edges, setEdges]);

  const handleNodeClick: NodeMouseHandler = (_, n) => {
    if (n.id.startsWith("__domain_")) return;
    onSelect(n.id);
  };

  const handleNodeDoubleClick: NodeMouseHandler = (_, n) => {
    if (n.type === "pipelineDagFrame") {
      setCollapsedDags(prev => {
        const next = new Set(prev);
        if (next.has(n.id)) next.delete(n.id);
        else next.add(n.id);
        return next;
      });
    }
  };

  return (
    <div className="relative w-full h-full border border-[var(--border-subtle)] rounded-[var(--radius-lg)] overflow-hidden" style={{ background: "var(--bg-base)" }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={handleNodeClick}
        onNodeDoubleClick={handleNodeDoubleClick}
        onPaneClick={() => onSelect(null)}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={{ maxZoom: 0.9, padding: 0.1 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.05}
        maxZoom={2}
        nodesDraggable
        elevateNodesOnSelect
      >
        <Background color="var(--border-subtle)" gap={28} size={1} />
        <Controls
          style={{ background: "var(--bg-surface)", borderColor: "var(--border-default)", borderRadius: "var(--radius-md)", overflow: "hidden" }}
        />
        <MiniMap
          nodeColor={(n) => minimapColor(((n.data as { node?: { type?: string } } | undefined)?.node?.type) ?? "")}
          className="hidden md:block"
          style={{ background: "var(--bg-surface)", border: "1px solid var(--border-subtle)", borderRadius: "var(--radius-md)" }}
        />
      </ReactFlow>
      {/* Hint */}
      <div className="absolute bottom-3 left-3 text-[10px] uppercase tracking-wider text-[var(--text-faint)] bg-[var(--bg-surface)]/80 border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2 py-1 backdrop-blur">
        Double-click a DAG frame to collapse / expand
      </div>
    </div>
  );
}
