"use client";

import { useRef, useState } from "react";
import { glpiAPI } from "@/lib/api";
import Button from "@/components/ui/Button";

// Each uploaded file becomes a GLPI Document with its own id. The answer value
// for a Formcreator file question is an array of these ids.
export interface UploadedDoc {
  id: number;
  filename: string;
  mime?: string;
  size: number;
}

interface Props {
  label?: string;
  error?: string;
  profileID: number | null;
  value: UploadedDoc[];
  onChange: (docs: UploadedDoc[]) => void;
  accept?: string; // comma-separated mime types / extensions (passed directly to <input accept>)
  maxFiles?: number;
  disabled?: boolean;
}

export default function FormcreatorFileInput({
  label,
  error,
  profileID,
  value,
  onChange,
  accept,
  maxFiles,
  disabled,
}: Props) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [uploading, setUploading] = useState(false);
  const [uploadErr, setUploadErr] = useState<string | null>(null);

  const atLimit = maxFiles != null && value.length >= maxFiles;

  const handlePick = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files ? Array.from(e.target.files) : [];
    if (!files.length || !profileID) return;
    setUploading(true);
    setUploadErr(null);
    try {
      // Sequential uploads keep the UI simple and avoid hammering the proxy;
      // Formcreator file questions rarely need more than a handful.
      const uploaded: UploadedDoc[] = [];
      for (const f of files) {
        if (maxFiles != null && value.length + uploaded.length >= maxFiles) break;
        const res = await glpiAPI.uploadFormFile(profileID, f);
        uploaded.push({ id: res.id, filename: res.filename, mime: res.mime, size: res.size });
      }
      onChange([...value, ...uploaded]);
    } catch (err) {
      setUploadErr(err instanceof Error ? err.message : "Falha no upload");
    } finally {
      setUploading(false);
      // Reset so picking the same file twice still fires onChange.
      if (inputRef.current) inputRef.current.value = "";
    }
  };

  const remove = (docID: number) => {
    onChange(value.filter((d) => d.id !== docID));
  };

  return (
    <div className="space-y-1.5">
      {label && (
        <label className="block text-xs font-medium text-[var(--text-secondary)] tracking-wide">
          {label}
        </label>
      )}
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        multiple={maxFiles == null || maxFiles > 1}
        className="hidden"
        onChange={handlePick}
        disabled={disabled || !profileID || atLimit}
      />
      <div className="flex items-center gap-2 flex-wrap">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={() => inputRef.current?.click()}
          loading={uploading}
          disabled={disabled || !profileID || atLimit}
        >
          {atLimit ? "Limite atingido" : "Adicionar arquivo"}
        </Button>
        {maxFiles != null && (
          <span className="text-[11px] text-[var(--text-faint)]">
            {value.length} / {maxFiles}
          </span>
        )}
        {accept && (
          <span className="text-[11px] text-[var(--text-faint)]" title={accept}>
            Tipos: {accept}
          </span>
        )}
      </div>
      {value.length > 0 && (
        <ul className="space-y-1">
          {value.map((doc) => (
            <li
              key={doc.id}
              className="flex items-center justify-between gap-2 bg-[var(--bg-elevated)] border border-[var(--border-subtle)] rounded-[var(--radius-sm)] px-2.5 py-1.5 text-xs"
            >
              <span className="truncate text-[var(--text-primary)]">
                {doc.filename}{" "}
                <span className="text-[var(--text-faint)]">· {formatBytes(doc.size)}</span>
              </span>
              {!disabled && (
                <button
                  type="button"
                  onClick={() => remove(doc.id)}
                  className="text-[var(--text-faint)] hover:text-red-400"
                  aria-label={`Remover ${doc.filename}`}
                >
                  ×
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
      {uploadErr && <p className="text-xs text-red-400">{uploadErr}</p>}
      {error && !uploadErr && <p className="text-xs text-red-400">{error}</p>}
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
