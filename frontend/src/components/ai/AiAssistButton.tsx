"use client";

import { useState } from "react";
import Button from "@/components/ui/Button";
import Icon from "@/components/ui/Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

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
      <Icon path={ICON_PATHS.sparkles} className="w-3.5 h-3.5 mr-1" />
      {label}
    </Button>
  );
}
