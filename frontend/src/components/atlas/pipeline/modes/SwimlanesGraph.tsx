"use client";

import { useEffect, useMemo } from "react";
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
import { layoutSwimlanes } from "@/lib/atlas/layoutSwimlanes";
import EntityNode from "../nodes/EntityNode";
import SwimlaneHeaderNode from "../nodes/SwimlaneHeaderNode";
import { minimapColor } from "@/lib/lineage/style";

const NODE_TYPES = {
  pipelineEntity: EntityNode,
  swimlaneHeader: SwimlaneHeaderNode,
};

interface Props {
  indexes: AtlasIndexes;
  filters: AtlasFilters;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
}

export default function SwimlanesGraph({ indexes, filters, selectedId, onSelect }: Props) {
  const layout = useMemo(() => layoutSwimlanes(indexes, filters), [indexes, filters]);
  const decorated = useMemo<Node[]>(() => layout.nodes.map(n => (
    n.id === selectedId ? { ...n, selected: true } : n
  )), [layout.nodes, selectedId]);
  const [nodes, setNodes, onNodesChange] = useNodesState(decorated);
  const [edges, setEdges, onEdgesChange] = useEdgesState(layout.edges);

  useEffect(() => setNodes(decorated), [decorated, setNodes]);
  useEffect(() => setEdges(layout.edges), [layout.edges, setEdges]);

  const handleNodeClick: NodeMouseHandler = (_, n) => {
    if (n.id.startsWith("__lane_")) return;
    onSelect(n.id);
  };

  return (
    <div className="relative w-full h-full border border-[var(--border-subtle)] rounded-[var(--radius-lg)] overflow-hidden" style={{ background: "var(--bg-base)" }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={handleNodeClick}
        onPaneClick={() => onSelect(null)}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={{ maxZoom: 1, padding: 0.15 }}
        proOptions={{ hideAttribution: true }}
        minZoom={0.05}
        maxZoom={2}
        nodesDraggable
        defaultEdgeOptions={{ animated: false }}
      >
        <Background color="var(--border-subtle)" gap={32} size={1} />
        <Controls
          style={{ background: "var(--bg-surface)", borderColor: "var(--border-default)", borderRadius: "var(--radius-md)", overflow: "hidden" }}
        />
        <MiniMap
          nodeColor={(n) => minimapColor(((n.data as { node?: { type?: string } } | undefined)?.node?.type) ?? "")}
          className="hidden md:block"
          style={{ background: "var(--bg-surface)", border: "1px solid var(--border-subtle)", borderRadius: "var(--radius-md)" }}
        />
      </ReactFlow>
    </div>
  );
}
