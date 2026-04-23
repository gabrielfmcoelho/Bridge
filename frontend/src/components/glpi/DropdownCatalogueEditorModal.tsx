"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery, useMutation } from "@tanstack/react-query";
import { glpiAPI, type GlpiCatalogueOption } from "@/lib/api";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import Button from "@/components/ui/Button";

interface Props {
  itemtype: string | null;
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}

// The scraper snippet admins paste into their GLPI browser DevTools. Collects
// rendered <option>s (both plain selects and select2-populated lists) and
// emits JSON that matches the GlpiCatalogueOption shape.
const SCRAPER_SNIPPET = `copy(JSON.stringify(
  [...document.querySelectorAll('select option, .select2-results__option')]
    .map(o => ({
      id: parseInt(o.value || o.dataset.id || '', 10),
      name: (o.textContent || '').trim(),
    }))
    .filter(o => Number.isFinite(o.id) && o.id > 0 && o.name),
  null, 2
));`;

export default function DropdownCatalogueEditorModal({ itemtype, open, onClose, onSaved }: Props) {
  const [text, setText] = useState("");
  const [parseErr, setParseErr] = useState<string | null>(null);
  const [saveErr, setSaveErr] = useState<string | null>(null);
  const [snippetCopied, setSnippetCopied] = useState(false);

  const { data, isFetching } = useQuery({
    queryKey: ["glpi-dropdown-catalogue", itemtype],
    queryFn: () => glpiAPI.getDropdownCatalogue(itemtype!),
    enabled: open && !!itemtype,
    retry: false,
  });

  // Seed the textarea when the catalogue arrives (or reset on close).
  useEffect(() => {
    if (!open) {
      setText("");
      setParseErr(null);
      setSaveErr(null);
      setSnippetCopied(false);
      return;
    }
    if (data?.options) {
      setText(JSON.stringify(data.options, null, 2));
    }
  }, [open, data]);

  // Live-validate so the admin sees errors without clicking Save.
  const parsed = useMemo<GlpiCatalogueOption[] | null>(() => {
    const raw = text.trim();
    if (!raw) {
      setParseErr(null);
      return [];
    }
    try {
      const obj = JSON.parse(raw);
      if (!Array.isArray(obj)) {
        setParseErr("Esperado um array JSON");
        return null;
      }
      const out: GlpiCatalogueOption[] = [];
      for (let i = 0; i < obj.length; i++) {
        const row = obj[i];
        if (!row || typeof row !== "object") {
          setParseErr(`Linha ${i + 1}: item inválido`);
          return null;
        }
        const id = typeof row.id === "number" ? row.id : parseInt(String(row.id ?? ""), 10);
        const name = typeof row.name === "string" ? row.name.trim() : "";
        if (!Number.isFinite(id) || id <= 0) {
          setParseErr(`Linha ${i + 1}: id inválido`);
          return null;
        }
        if (!name) {
          setParseErr(`Linha ${i + 1}: name vazio`);
          return null;
        }
        const clean: GlpiCatalogueOption = { id, name };
        if (typeof row.completename === "string" && row.completename.trim()) {
          clean.completename = row.completename.trim();
        }
        if (row.parent_id != null) {
          const p = typeof row.parent_id === "number" ? row.parent_id : parseInt(String(row.parent_id), 10);
          if (Number.isFinite(p) && p > 0) clean.parent_id = p;
        }
        out.push(clean);
      }
      setParseErr(null);
      return out;
    } catch (e) {
      setParseErr(e instanceof Error ? e.message : "JSON inválido");
      return null;
    }
  }, [text]);

  const upsertMutation = useMutation({
    mutationFn: () => glpiAPI.upsertDropdownCatalogue(itemtype!, parsed ?? []),
    onSuccess: () => onSaved(),
    onError: (err: Error) => setSaveErr(err.message),
  });

  const copySnippet = async () => {
    try {
      await navigator.clipboard.writeText(SCRAPER_SNIPPET);
      setSnippetCopied(true);
      setTimeout(() => setSnippetCopied(false), 2000);
    } catch {
      // No clipboard access — the <pre> is selectable so fallback is manual copy.
    }
  };

  return (
    <ResponsiveModal
      open={open}
      onClose={onClose}
      title={itemtype ? `Catálogo: ${itemtype}` : "Catálogo"}
    >
      <div className="space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-[2fr_1fr] gap-4">
          {/* Left: JSON textarea */}
          <div className="space-y-1.5">
            <label className="block text-xs font-medium text-[var(--text-secondary)]">
              Opções (JSON)
            </label>
            <textarea
              value={text}
              onChange={(e) => setText(e.target.value)}
              rows={16}
              spellCheck={false}
              className="w-full bg-[var(--bg-elevated)] text-[var(--text-primary)] border border-[var(--border-default)] rounded-[var(--radius-md)] px-3 py-2 text-[12px] focus:outline-none focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
              style={{ fontFamily: "var(--font-mono)" }}
              placeholder={'[\n  { "id": 1, "name": "…", "completename": "A > B > …" }\n]'}
              disabled={isFetching}
            />
            <p className="text-[11px] text-[var(--text-muted)]">
              {parseErr ? (
                <span className="text-red-400">⚠ {parseErr}</span>
              ) : parsed ? (
                <>✓ {parsed.length} opção{parsed.length === 1 ? "" : "es"} prontas para salvar</>
              ) : (
                <>Cole o JSON — lista de <code>{"{ id, name, completename?, parent_id? }"}</code></>
              )}
            </p>
          </div>

          {/* Right: scraper snippet + help */}
          <aside className="space-y-3 text-xs text-[var(--text-muted)]">
            <div>
              <p className="font-semibold text-[var(--text-primary)] mb-1">Como importar</p>
              <ol className="list-decimal list-inside space-y-1">
                <li>
                  Abra no GLPI qualquer página que mostre o dropdown de{" "}
                  <code>{itemtype}</code> (ex.: Administração ›{" "}
                  {itemtype === "ITILCategory" ? "Categorias ITIL" : itemtype}).
                </li>
                <li>Abra o DevTools (F12), aba <strong>Console</strong>.</li>
                <li>Cole o snippet abaixo e aperte Enter. Ele copia o JSON pronto.</li>
                <li>Cole aqui no textarea à esquerda e clique Salvar.</li>
              </ol>
            </div>
            <div>
              <div className="flex items-center justify-between gap-2 mb-1">
                <span className="text-[10px] uppercase tracking-wider text-[var(--text-faint)]">
                  Snippet
                </span>
                <button
                  type="button"
                  onClick={copySnippet}
                  className="text-[11px] text-[var(--accent)] hover:underline"
                >
                  {snippetCopied ? "Copiado ✓" : "Copiar"}
                </button>
              </div>
              <pre className="whitespace-pre-wrap break-all bg-[var(--bg-overlay)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] p-2 text-[10.5px] leading-tight" style={{ fontFamily: "var(--font-mono)" }}>
                {SCRAPER_SNIPPET}
              </pre>
            </div>
            <p>
              Alternativa: exporte o itemtype do GLPI como CSV (Administração ›
              exportação) e converta para JSON — o formato esperado é array de{" "}
              <code>{"{ id, name }"}</code>.
            </p>
          </aside>
        </div>

        {saveErr && (
          <div className="rounded-[var(--radius-md)] border border-red-500/30 bg-red-500/10 text-red-300 text-sm px-3 py-2">
            {saveErr}
          </div>
        )}

        <div className="flex items-center justify-end gap-2 pt-2 border-t border-[var(--border-default)]">
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancelar
          </Button>
          <Button
            type="button"
            onClick={() => {
              setSaveErr(null);
              upsertMutation.mutate();
            }}
            loading={upsertMutation.isPending}
            disabled={!parsed || parseErr != null}
          >
            Salvar catálogo
          </Button>
        </div>
      </div>
    </ResponsiveModal>
  );
}
