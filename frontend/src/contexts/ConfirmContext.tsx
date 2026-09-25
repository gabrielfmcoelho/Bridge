"use client";

import { createContext, useCallback, useContext, useRef, useState, type ReactNode } from "react";
import { useLocale } from "./LocaleContext";
import ResponsiveModal from "@/components/ui/ResponsiveModal";
import FormFooter from "@/components/ui/FormFooter";
import Input from "@/components/ui/Input";

export interface ConfirmOptions {
  title: string;
  message?: ReactNode;
  /** Primary button text; defaults to "Confirmar". */
  confirmLabel?: string;
  /** Secondary button text; defaults to "Cancelar" (a yes/no question passes "Não"). */
  cancelLabel?: string;
  /** Destructive: the primary button is the danger variant. */
  danger?: boolean;
  /** The admin must type this exactly to enable the button (irreversible actions). */
  requireText?: string;
}

type Confirm = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<Confirm | null>(null);

/**
 * `const confirm = useConfirm(); if (await confirm({ title, danger: true })) …`
 * A drop-in for window.confirm: a real dialog (focus trap, Escape, labelled
 * title, translated buttons) that resolves true only on the primary button.
 */
export function useConfirm(): Confirm {
  const ctx = useContext(ConfirmContext);
  if (!ctx) throw new Error("useConfirm needs <ConfirmProvider>");
  return ctx;
}

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const { t } = useLocale();
  const [options, setOptions] = useState<ConfirmOptions | null>(null);
  const [typed, setTyped] = useState("");
  const resolver = useRef<((ok: boolean) => void) | undefined>(undefined);

  const confirm = useCallback<Confirm>((o) => {
    resolver.current?.(false); // a second request cancels a pending one
    setTyped("");
    setOptions(o);
    return new Promise<boolean>((resolve) => { resolver.current = resolve; });
  }, []);

  const close = (ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = undefined;
    setOptions(null);
  };

  const blocked = !!options?.requireText && typed !== options.requireText;

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      <ResponsiveModal
        open={!!options}
        onClose={() => close(false)}
        title={options?.title}
        size="sm"
        footer={
          <FormFooter
            onCancel={() => close(false)}
            cancelLabel={options?.cancelLabel}
            submitLabel={options?.confirmLabel ?? t("common.confirm")}
            onSubmit={() => close(true)}
            variant={options?.danger ? "danger" : "primary"}
            disabled={blocked}
          />
        }
      >
        <div className="space-y-4">
          {options?.message && <div className="text-sm text-[var(--text-secondary)]">{options.message}</div>}
          {options?.requireText && (
            <Input
              label={t("confirm.typeToConfirm", { text: options.requireText })}
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              autoComplete="off"
              autoFocus
            />
          )}
        </div>
      </ResponsiveModal>
    </ConfirmContext.Provider>
  );
}
