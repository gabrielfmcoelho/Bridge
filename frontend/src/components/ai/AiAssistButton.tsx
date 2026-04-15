"use client";

import { useState } from "react";
import Button from "@/components/ui/Button";

interface AiAssistButtonProps {
  onGenerate: () => Promise<string>;
  onResult: (text: string) => void;
  label?: string;
}

export default function AiAssistButton({ onGenerate, onResult, label = "AI Assist" }: AiAssistButtonProps) {
  const [loading, setLoading] = useState(false);

  const handleClick = async () => {
    setLoading(true);
    try {
      const result = await onGenerate();
      onResult(result);
    } catch {
      // silently fail — the caller can handle errors
    } finally {
      setLoading(false);
    }
  };

  return (
    <Button
      type="button"
      variant="secondary"
      size="sm"
      onClick={handleClick}
      loading={loading}
    >
      <svg className="w-3.5 h-3.5 mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
        <path strokeLinecap="round" strokeLinejoin="round" d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z" />
      </svg>
      {label}
    </Button>
  );
}
