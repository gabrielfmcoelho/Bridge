"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useLocale } from "@/contexts/LocaleContext";
import { useTheme } from "@/contexts/ThemeContext";
import { marked } from "marked";
import DOMPurify from "isomorphic-dompurify";
import IconButton from "@/components/ui/IconButton";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

marked.setOptions({ breaks: true, gfm: true });

// renderMarkdown is the single sanitized markdown → HTML pipeline. marked.parse
// output is passed through DOMPurify before it ever reaches
// the DOM — mandatory because share/wiki content is rendered on
// the public, unauthenticated redeem page.
export function renderMarkdown(content: string): string {
  return DOMPurify.sanitize(marked.parse(content) as string);
}

// Outline fences diagrams as ```mermaidjs; plain ```mermaid is the common spelling.
const MERMAID_SELECTOR = "pre > code.language-mermaid, pre > code.language-mermaidjs";
let mermaidSeq = 0;

const ZOOM_MIN = 0.25;
const ZOOM_MAX = 4;
const clampZoom = (z: number) => Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, z));

// MermaidCanvas is one rendered diagram in a pannable, zoomable viewport:
// +/- buttons, Ctrl/Cmd + wheel, drag to pan. Zoom resizes the SVG (vector, so
// it stays crisp) and the viewport's own scrollbars do the panning.
function MermaidCanvas({ svg }: { svg: string }) {
  const { t } = useLocale();
  const viewport = useRef<HTMLDivElement>(null);
  const content = useRef<HTMLDivElement>(null);
  const natural = useRef(0);
  const [zoom, setZoom] = useState(1);

  // mermaid's SVG output (strict mode, sanitized by mermaid itself). Set once
  // imperatively so React never re-applies it over the zoom styling.
  useLayoutEffect(() => {
    const host = content.current!;
    host.innerHTML = svg;
    const el = host.querySelector("svg");
    natural.current = el ? parseFloat(el.style.maxWidth) || el.getBoundingClientRect().width : 0;
  }, [svg]);

  useLayoutEffect(() => {
    const el = content.current?.querySelector("svg");
    if (!el || !natural.current) return;
    el.style.maxWidth = "none";
    el.style.width = `${natural.current * zoom}px`;
    el.style.height = "auto";
  }, [zoom, svg]);

  // Non-passive so Ctrl + wheel zooms the diagram instead of the browser page.
  // A plain wheel keeps scrolling the page.
  useEffect(() => {
    const vp = viewport.current!;
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return;
      e.preventDefault();
      setZoom((z) => clampZoom(z * (e.deltaY < 0 ? 1.1 : 1 / 1.1)));
    };
    vp.addEventListener("wheel", onWheel, { passive: false });
    return () => vp.removeEventListener("wheel", onWheel);
  }, []);

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.button !== 0) return;
    const vp = e.currentTarget;
    const start = { x: e.clientX, y: e.clientY, left: vp.scrollLeft, top: vp.scrollTop };
    vp.setPointerCapture(e.pointerId);
    const move = (ev: PointerEvent) => {
      vp.scrollLeft = start.left - (ev.clientX - start.x);
      vp.scrollTop = start.top - (ev.clientY - start.y);
    };
    const up = () => {
      vp.removeEventListener("pointermove", move);
      vp.removeEventListener("pointerup", up);
      vp.removeEventListener("pointercancel", up);
    };
    vp.addEventListener("pointermove", move);
    vp.addEventListener("pointerup", up);
    vp.addEventListener("pointercancel", up);
  };

  return (
    <div className="my-2 rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-surface)]">
      <div className="flex items-center gap-1 border-b border-[var(--border-subtle)] px-2 py-1">
        <span className="mr-auto text-2xs text-[var(--text-faint)]">{t("common.diagramHint")}</span>
        <IconButton label={t("common.zoomOut")} disabled={zoom <= ZOOM_MIN} onClick={() => setZoom((z) => clampZoom(z / 1.25))}>
          <Icon path={ICON_PATHS.minus} />
        </IconButton>
        <span className="w-10 text-center text-2xs tabular-nums text-[var(--text-muted)]">{Math.round(zoom * 100)}%</span>
        <IconButton label={t("common.zoomIn")} disabled={zoom >= ZOOM_MAX} onClick={() => setZoom((z) => clampZoom(z * 1.25))}>
          <Icon path={ICON_PATHS.plus} />
        </IconButton>
        <IconButton label={t("common.zoomReset")} disabled={zoom === 1} onClick={() => setZoom(1)}>
          <Icon path={ICON_PATHS.refresh} />
        </IconButton>
      </div>
      <div
        ref={viewport}
        onPointerDown={onPointerDown}
        className="max-h-[70vh] cursor-grab touch-none overflow-auto p-3 select-none active:cursor-grabbing"
      >
        <div ref={content} className="w-max" />
      </div>
    </div>
  );
}

// MarkdownHtml renders already-sanitized markdown HTML, then upgrades mermaid
// code blocks to zoomable SVG canvases (portals into hosts placed after each
// block). mermaid is imported only when a block is present. The original <pre>
// is hidden, not replaced, so a re-run (theme flip, strict-mode double effect)
// can undo its own work and render again from the source text.
function MarkdownHtml({ html, className }: { html: string; className: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const { theme } = useTheme();
  const { t } = useLocale();
  const errorNote = t("common.diagramError");
  const [diagrams, setDiagrams] = useState<{ host: HTMLElement; svg: string }[]>([]);

  // innerHTML is owned here, not by dangerouslySetInnerHTML: React re-applied
  // the markup on re-renders, wiping the inserted diagrams.
  useLayoutEffect(() => {
    if (ref.current) ref.current.innerHTML = html;
  }, [html]);

  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const blocks = Array.from(root.querySelectorAll<HTMLElement>(MERMAID_SELECTOR));
    if (blocks.length === 0) return;
    let cancelled = false;
    const added: Element[] = [];

    (async () => {
      const { default: mermaid } = await import("mermaid");
      if (cancelled) return;
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: theme === "dark" ? "dark" : "default",
      });
      const rendered: { host: HTMLElement; svg: string }[] = [];
      for (const code of blocks) {
        const pre = code.parentElement!;
        const id = `mermaid-${++mermaidSeq}`;
        const out = document.createElement("div");
        try {
          // textContent, never innerHTML: the diagram source is untrusted text.
          const { svg } = await mermaid.render(id, code.textContent ?? "");
          if (cancelled) return;
          rendered.push({ host: out, svg });
          pre.hidden = true;
        } catch {
          document.getElementById(`d${id}`)?.remove(); // mermaid's leftover scratch node
          if (cancelled) return;
          out.className = "text-2xs text-[var(--text-muted)]";
          out.textContent = errorNote;
        }
        pre.after(out);
        added.push(out);
      }
      setDiagrams(rendered);
    })().catch(() => {}); // a failed chunk load leaves the code blocks as-is

    return () => {
      cancelled = true;
      setDiagrams([]);
      added.forEach((n) => n.remove());
      blocks.forEach((c) => (c.parentElement!.hidden = false));
    };
  }, [html, theme, errorNote]);

  return (
    <>
      <div ref={ref} className={className} />
      {diagrams.map((d, i) => createPortal(<MermaidCanvas svg={d.svg} />, d.host, String(i)))}
    </>
  );
}

interface MarkdownEditorProps {
  value: string;
  onChange: (value: string) => void;
  label?: string;
  rows?: number;
  placeholder?: string;
}

export default function MarkdownEditor({ value, onChange, label, rows = 5, placeholder }: MarkdownEditorProps) {
  const { t } = useLocale();
  const [mode, setMode] = useState<"write" | "preview">("write");

  return (
    <div>
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide mb-1.5">{label}</label>
      )}
      <div className="border border-[var(--border-default)] rounded-[var(--radius-md)] overflow-hidden bg-[var(--bg-elevated)]">
        {/* Tab bar */}
        <div className="flex border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]">
          <button
            type="button"
            onClick={() => setMode("write")}
            className={`px-3 py-1.5 text-xs font-medium transition-colors ${
              mode === "write"
                ? "text-[var(--accent)] border-b-2 border-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            }`}
          >
            {t("common.write")}
          </button>
          <button
            type="button"
            onClick={() => setMode("preview")}
            className={`px-3 py-1.5 text-xs font-medium transition-colors ${
              mode === "preview"
                ? "text-[var(--accent)] border-b-2 border-[var(--accent)]"
                : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
            }`}
          >
            {t("common.preview")}
          </button>
        </div>

        {mode === "write" ? (
          <textarea
            value={value}
            onChange={(e) => onChange(e.target.value)}
            rows={rows}
            placeholder={placeholder}
            className="w-full bg-transparent px-3 py-2 text-sm text-[var(--text-primary)] placeholder-[var(--text-faint)] focus:outline-none resize-none font-mono"
            style={{ fontSize: "13px" }}
          />
        ) : (
          <MarkdownHtml
            className="markdown-preview px-3 py-2 text-sm text-[var(--text-primary)] min-h-[80px]"
            html={value ? renderMarkdown(value) : '<span class="text-[var(--text-faint)]">Nothing to preview</span>'}
          />
        )}
      </div>
    </div>
  );
}

export function MarkdownContent({ content }: { content: string }) {
  if (!content) return null;
  return <MarkdownHtml className="markdown-preview text-sm text-[var(--text-secondary)]" html={renderMarkdown(content)} />;
}

// SafeMarkdownContent is the same sanitized renderer, named explicitly for use on
// the public redeem page. className overridable so share styling can differ from
// the in-app wiki viewer.
export function SafeMarkdownContent({
  content,
  className = "markdown-preview text-sm text-[var(--text-primary)]",
}: {
  content: string;
  className?: string;
}) {
  if (!content) return null;
  return <MarkdownHtml className={className} html={renderMarkdown(content)} />;
}
