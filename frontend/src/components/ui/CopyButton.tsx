"use client";

import Button from "./Button";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";
import { useCopy } from "@/hooks/useCopy";
import { useLocale } from "@/contexts/LocaleContext";

interface CopyButtonProps extends Omit<React.ComponentProps<typeof Button>, "onClick" | "children"> {
  value: string;
  label?: string;
  copiedLabel?: string;
  /** Show the copy/check glyph before the label. */
  icon?: boolean;
}

// Button + useCopy for the plain "Copy / Copied" case. Sites that share one
// copied state between several controls use the hook directly.
export default function CopyButton({ value, label, copiedLabel, icon = false, variant = "secondary", ...props }: CopyButtonProps) {
  const { t } = useLocale();
  const { copied, copy } = useCopy();
  return (
    <Button type="button" variant={variant} onClick={() => copy(value)} {...props}>
      {icon && <Icon path={copied ? ICON_PATHS.check : ICON_PATHS.copy} className={copied ? "w-4 h-4 text-[var(--success)]" : "w-4 h-4"} />}
      {copied ? copiedLabel ?? t("common.copied") : label ?? t("common.copy")}
    </Button>
  );
}
