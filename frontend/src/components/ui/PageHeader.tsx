"use client";

import Button from "./Button";
import IconButton from "./IconButton";

interface PageHeaderProps {
  title: string;
  addLabel?: string;
  onAdd?: () => void;
}

export default function PageHeader({ title, addLabel, onAdd }: PageHeaderProps) {
  return (
    <div className="flex items-center justify-between gap-2 mb-6">
      <h1 className="text-2xl font-bold" style={{ fontFamily: "var(--font-display)" }}>{title}</h1>
      {onAdd && addLabel && (
        <>
          <div className="hidden sm:block">
            <Button onClick={onAdd}>+ {addLabel}</Button>
          </div>
          <IconButton
            variant="accent"
            size="md"
            onClick={onAdd}
            title={addLabel}
            className="sm:hidden"
          >
            <svg className="w-4.5 h-4.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
            </svg>
          </IconButton>
        </>
      )}
    </div>
  );
}
