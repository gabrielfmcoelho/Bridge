"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  ReactFlow, ReactFlowProvider, Background, Controls, MiniMap, Panel, ConnectionMode, MarkerType,
  addEdge, useNodesState, useEdgesState, useReactFlow, type Connection, type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useQueryClient } from "@tanstack/react-query";
import { ApiError, canvasAPI, globalIssuesAPI } from "@/lib/api";
import type { Canvas } from "@/lib/types";
import { useLocale } from "@/contexts/LocaleContext";
import { useFlag } from "@/contexts/FlagContext";
import { ICON_PATHS } from "@/lib/icon-paths";
import Icon from "@/components/ui/Icon";
import Button from "@/components/ui/Button";
import { issuesKey } from "@/components/issues/IssuesBoard";
import { nodeTypes, ReadOnlyContext, TONES, toneDot, type CanvasNode, type ShapeKind, type Tone } from "./nodes";

type SaveState = "saved" | "saving" | "conflict" | "error";

const edgeDefaults = { markerEnd: { type: MarkerType.ArrowClosed }, style: { strokeWidth: 1.5 } };

// What gets persisted: xyflow's per-session flags (selection, drag, measured
// size) stay out so clicking around doesn't trigger a save.
function serialize(nodes: CanvasNode[], edges: Edge[]) {
  return JSON.stringify({
    nodes: nodes.map(({ id, type, position, width, height, data }) => ({ id, type, position, width, height, data })),
    edges: edges.map(({ id, source, target, sourceHandle, targetHandle, label }) => ({ id, source, target, sourceHandle, targetHandle, label })),
  });
}

type BoardProps = {
  canvas: Canvas;
  canEdit: boolean;
  /** Shared with the page so a rename sends the current version too. */
  version: React.MutableRefObject<number>;
};

function Board({ canvas, canEdit, version }: BoardProps) {
  const { t } = useLocale();
  const flag = useFlag();
  const qc = useQueryClient();
  const { screenToFlowPosition } = useReactFlow();
  const wrapper = useRef<HTMLDivElement>(null);

  const initial = canvas.content ?? { nodes: [], edges: [] };
  const [nodes, setNodes, onNodesChange] = useNodesState<CanvasNode>(initial.nodes as CanvasNode[]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>((initial.edges as Edge[]).map((e) => ({ ...e, ...edgeDefaults })));
  const [save, setSave] = useState<SaveState>("saved");
  const [sending, setSending] = useState(false);
  const saved = useRef(serialize(nodes, edges));

  // Autosave: debounce, then PUT with the loaded version. A 409 stops saving
  // until reload — never overwrite someone else's edit.
  useEffect(() => {
    if (!canEdit || save === "conflict") return;
    const body = serialize(nodes, edges);
    if (body === saved.current) return;
    const timer = setTimeout(async () => {
      setSave("saving");
      try {
        const res = await canvasAPI.update(canvas.id, { version: version.current, content: JSON.parse(body) });
        version.current = res.version;
        saved.current = body;
        setSave("saved");
      } catch (e) {
        if (e instanceof ApiError && e.status === 409) {
          setSave("conflict");
          flag({ appearance: "error", title: t("canvas.conflictTitle"), description: t("canvas.conflictHint"),
            actions: [{ label: t("canvas.reload"), onClick: () => window.location.reload() }] });
        } else {
          setSave("error");
          flag({ appearance: "error", title: t("canvas.saveFailed"), description: e instanceof Error ? e.message : undefined });
        }
      }
    }, 800);
    return () => clearTimeout(timer);
  }, [nodes, edges, canEdit, save, canvas.id, flag, t, version]);

  // Leaving with unsaved changes asks first.
  useEffect(() => {
    const warn = (e: BeforeUnloadEvent) => { if (serialize(nodes, edges) !== saved.current) e.preventDefault(); };
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [nodes, edges]);

  const onConnect = useCallback((c: Connection) => setEdges((es) => addEdge({ ...c, ...edgeDefaults }, es)), [setEdges]);

  const add = (type: CanvasNode["type"], at?: { x: number; y: number }, shape?: ShapeKind) => {
    const rect = wrapper.current?.getBoundingClientRect();
    const pos = at ?? screenToFlowPosition({ x: (rect?.left ?? 0) + (rect?.width ?? 0) / 2, y: (rect?.top ?? 0) + (rect?.height ?? 0) / 2 });
    const size = type === "note" ? { width: 200, height: 120 } : type === "text" ? { width: 200, height: 40 } : { width: 160, height: 100 };
    const node: CanvasNode = {
      id: crypto.randomUUID(), type, ...size,
      position: { x: pos.x - size.width / 2, y: pos.y - size.height / 2 },
      data: { text: "", ...(shape ? { shape } : {}) },
      selected: true,
    };
    setNodes((ns) => [...ns.map((n) => ({ ...n, selected: false })), node]);
  };

  const selected = nodes.filter((n) => n.selected);
  const one = selected.length === 1 ? selected[0] : null;
  const setTone = (tone: Tone) => setNodes((ns) => ns.map((n) => (n.selected ? { ...n, data: { ...n.data, tone } } : n)));

  const sendToBacklog = async () => {
    if (!one || !one.data.text.trim()) return;
    const [first, ...rest] = one.data.text.trim().split("\n");
    setSending(true);
    try {
      const issue = await globalIssuesAPI.create({
        entity_type: "entidade", entity_id: canvas.entidade_id,
        title: first.slice(0, 200), description: rest.join("\n").trim(),
        source: "canvas", source_ref: `canvas:${canvas.id}/${one.id}`,
      });
      setNodes((ns) => ns.map((n) => (n.id === one.id ? { ...n, data: { ...n.data, issueId: issue.id } } : n)));
      qc.invalidateQueries({ queryKey: issuesKey("entidade", canvas.entidade_id) });
      flag({ appearance: "success", title: t("canvas.sentTitle"), description: t("canvas.sentHint", { entidade: canvas.entidade_name }) });
    } catch (e) {
      flag({ appearance: "error", title: t("canvas.sendFailed"), description: e instanceof Error ? e.message : undefined });
    } finally {
      setSending(false);
    }
  };

  const tool = (label: string, path: string, onClick: () => void) => (
    <button type="button" onClick={onClick} title={label} aria-label={label}
      className="w-8 h-8 inline-flex items-center justify-center rounded-[var(--radius-sm)] text-[var(--text-secondary)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)]">
      <Icon path={path} />
    </button>
  );

  return (
    <ReadOnlyContext.Provider value={!canEdit}>
      <div ref={wrapper} className="h-[calc(100dvh-12rem)] min-h-[24rem] rounded-[var(--radius-lg)] border border-[var(--border-default)] overflow-hidden"
        onDoubleClick={(e) => {
          if (canEdit && (e.target as HTMLElement).classList.contains("react-flow__pane")) add("note", screenToFlowPosition({ x: e.clientX, y: e.clientY }));
        }}>
        <ReactFlow
          nodes={nodes} edges={edges} nodeTypes={nodeTypes}
          onNodesChange={onNodesChange} onEdgesChange={onEdgesChange} onConnect={onConnect}
          connectionMode={ConnectionMode.Loose}
          nodesDraggable={canEdit} nodesConnectable={canEdit} edgesReconnectable={canEdit}
          deleteKeyCode={canEdit ? ["Backspace", "Delete"] : null}
          zoomOnDoubleClick={false}
          fitView fitViewOptions={{ maxZoom: 1, padding: 0.3 }}
          proOptions={{ hideAttribution: true }}
          style={{ background: "var(--bg-base)" }}
        >
          <Background color="var(--border-subtle)" gap={24} size={1} />
          <Controls style={{ background: "var(--bg-surface)", borderColor: "var(--border-default)", borderRadius: "var(--radius-md)", overflow: "hidden" }} />
          <MiniMap pannable zoomable style={{ background: "var(--bg-surface)" }} maskColor="var(--bg-overlay)" />

          {canEdit && (
            <Panel position="top-left">
              <div className="flex flex-wrap items-center gap-1 rounded-[var(--radius-md)] border border-[var(--border-default)] bg-[var(--bg-surface)] p-1 shadow-sm">
                {tool(t("canvas.addNote"), ICON_PATHS.document, () => add("note"))}
                {tool(t("canvas.addText"), ICON_PATHS.editPencil, () => add("text"))}
                {tool(t("canvas.addRect"), ICON_PATHS.viewCards, () => add("shape", undefined, "rect"))}
                {tool(t("canvas.addEllipse"), ICON_PATHS.circle, () => add("shape", undefined, "ellipse"))}
                {tool(t("canvas.addDiamond"), ICON_PATHS.cube, () => add("shape", undefined, "diamond"))}
                {selected.length > 0 && (
                  <>
                    <span className="mx-1 h-5 w-px bg-[var(--border-default)]" />
                    {TONES.map((tone) => (
                      <button key={tone} type="button" onClick={() => setTone(tone)} aria-label={t(`canvas.tone.${tone}`)} title={t(`canvas.tone.${tone}`)}
                        className="w-6 h-6 inline-flex items-center justify-center rounded-full hover:bg-[var(--bg-elevated)]">
                        <span className={`w-3.5 h-3.5 rounded-full ${toneDot[tone]}`} />
                      </button>
                    ))}
                    {tool(t("common.delete"), ICON_PATHS.trashOutline, () => {
                      const ids = new Set(selected.map((n) => n.id));
                      setNodes((ns) => ns.filter((n) => !ids.has(n.id)));
                      setEdges((es) => es.filter((e) => !ids.has(e.source) && !ids.has(e.target)));
                    })}
                  </>
                )}
                {one && !one.data.issueId && one.data.text.trim() && (
                  <Button size="sm" variant="secondary" loading={sending} onClick={sendToBacklog}>
                    <Icon path={ICON_PATHS.send} size="xs" /> {t("canvas.sendToBacklog")}
                  </Button>
                )}
              </div>
            </Panel>
          )}
          <Panel position="top-right">
            <span className={`text-xs ${save === "conflict" || save === "error" ? "text-[var(--danger)]" : "text-[var(--text-muted)]"}`}>
              {canEdit ? t(`canvas.save.${save}`) : t("canvas.readOnly")}
            </span>
          </Panel>
          {nodes.length === 0 && (
            <Panel position="bottom-center">
              <span className="text-xs text-[var(--text-muted)]">{canEdit ? t("canvas.emptyBoardHint") : t("canvas.emptyBoard")}</span>
            </Panel>
          )}
        </ReactFlow>
      </div>
    </ReadOnlyContext.Provider>
  );
}

export default function CanvasBoard(props: BoardProps) {
  return (
    <ReactFlowProvider>
      <Board {...props} />
    </ReactFlowProvider>
  );
}
