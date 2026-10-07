"use client";

import { createContext, useContext, useState } from "react";
import { Handle, NodeResizer, Position, useReactFlow, type Node, type NodeProps } from "@xyflow/react";
import { useLocale } from "@/contexts/LocaleContext";

export type Tone = "warning" | "info" | "success" | "accent" | "danger";
export const TONES: Tone[] = ["warning", "info", "success", "accent", "danger"];
export type ShapeKind = "rect" | "ellipse" | "diamond";

export type CanvasNodeData = {
  text: string;
  tone?: Tone;
  shape?: ShapeKind;
  /** Set once the item was sent to the entidade's backlog. */
  issueId?: number;
};
export type CanvasNode = Node<CanvasNodeData, "note" | "text" | "shape">;

/** Viewers see the board but can't edit text. */
export const ReadOnlyContext = createContext(false);

// Literal class strings so Tailwind sees every one.
const toneBg: Record<Tone, string> = {
  warning: "bg-[var(--warning)]/15 border-[var(--warning)]/40",
  info: "bg-[var(--info)]/15 border-[var(--info)]/40",
  success: "bg-[var(--success)]/15 border-[var(--success)]/40",
  accent: "bg-[var(--accent)]/15 border-[var(--accent)]/40",
  danger: "bg-[var(--danger)]/15 border-[var(--danger)]/40",
};
export const toneDot: Record<Tone, string> = {
  warning: "bg-[var(--warning)]",
  info: "bg-[var(--info)]",
  success: "bg-[var(--success)]",
  accent: "bg-[var(--accent)]",
  danger: "bg-[var(--danger)]",
};

// Loose connection mode: every handle is a source and connects to any other.
function Handles() {
  const cls = "!w-2 !h-2 !bg-[var(--text-faint)] !border-0 opacity-0 group-hover:opacity-100";
  return (
    <>
      <Handle id="t" type="source" position={Position.Top} className={cls} />
      <Handle id="r" type="source" position={Position.Right} className={cls} />
      <Handle id="b" type="source" position={Position.Bottom} className={cls} />
      <Handle id="l" type="source" position={Position.Left} className={cls} />
    </>
  );
}

/** The text, edited in place on double-click. */
function EditableText({ id, text, className = "" }: { id: string; text: string; className?: string }) {
  const { t } = useLocale();
  const readOnly = useContext(ReadOnlyContext);
  const { updateNodeData } = useReactFlow();
  const [editing, setEditing] = useState(false);
  if (editing && !readOnly) {
    return (
      <textarea
        autoFocus
        defaultValue={text}
        aria-label={t("canvas.editText")}
        onFocus={(e) => e.currentTarget.select()}
        onBlur={(e) => { updateNodeData(id, { text: e.currentTarget.value }); setEditing(false); }}
        onKeyDown={(e) => { if (e.key === "Escape") e.currentTarget.blur(); }}
        className={`nodrag nowheel w-full h-full resize-none bg-transparent outline-none text-sm text-[var(--text-primary)] ${className}`}
      />
    );
  }
  return (
    <div onDoubleClick={() => setEditing(true)}
      className={`w-full h-full overflow-hidden whitespace-pre-wrap break-words text-sm text-[var(--text-primary)] ${className}`}>
      {text || <span className="text-[var(--text-faint)]">{t("canvas.emptyNote")}</span>}
    </div>
  );
}

function BacklogBadge({ issueId }: { issueId?: number }) {
  const { t } = useLocale();
  if (!issueId) return null;
  return (
    <span className="absolute -top-2 right-2 rounded-full px-1.5 py-0.5 text-2xs font-medium bg-[var(--success)] text-[var(--bg-base)]">
      {t("canvas.inBacklog", { id: String(issueId) })}
    </span>
  );
}

function NoteNode({ id, data, selected }: NodeProps<CanvasNode>) {
  return (
    <div className={`group relative w-full h-full rounded-[var(--radius-md)] border p-3 shadow-sm ${toneBg[data.tone ?? "warning"]}`}>
      <NodeResizer isVisible={selected} minWidth={120} minHeight={60} />
      <Handles />
      <BacklogBadge issueId={data.issueId} />
      <EditableText id={id} text={data.text} />
    </div>
  );
}

function TextNode({ id, data, selected }: NodeProps<CanvasNode>) {
  return (
    <div className={`group relative w-full h-full p-1 ${selected ? "outline outline-1 outline-[var(--accent)]/50" : ""}`}>
      <NodeResizer isVisible={selected} minWidth={60} minHeight={24} />
      <Handles />
      <BacklogBadge issueId={data.issueId} />
      <EditableText id={id} text={data.text} className="!text-base font-medium" />
    </div>
  );
}

const shapeClip: Record<ShapeKind, string> = {
  rect: "rounded-[var(--radius-md)]",
  ellipse: "rounded-[50%]",
  diamond: "[clip-path:polygon(50%_0,100%_50%,50%_100%,0_50%)]",
};

function ShapeNode({ id, data, selected }: NodeProps<CanvasNode>) {
  const shape = data.shape ?? "rect";
  return (
    <div className="group relative w-full h-full">
      <NodeResizer isVisible={selected} minWidth={60} minHeight={40} />
      <Handles />
      {/* Border via the filled layer so the diamond clip keeps an outline-ish edge. */}
      <div className={`absolute inset-0 border-2 ${toneBg[data.tone ?? "info"]} ${shapeClip[shape]}`} />
      <BacklogBadge issueId={data.issueId} />
      <div className={`relative w-full h-full flex items-center justify-center text-center ${shape === "diamond" ? "px-[22%] py-[18%]" : "p-3"}`}>
        <EditableText id={id} text={data.text} className="flex items-center justify-center text-center" />
      </div>
    </div>
  );
}

export const nodeTypes = { note: NoteNode, text: TextNode, shape: ShapeNode };
