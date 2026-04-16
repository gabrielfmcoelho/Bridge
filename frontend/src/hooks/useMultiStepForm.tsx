"use client";

import { useEffect, useRef, type ReactNode } from "react";
import StepIndicator from "@/components/ui/StepIndicator";
import Button from "@/components/ui/Button";

interface UseMultiStepFormOptions {
  step: number;
  totalSteps: number;
  stepLabels: string[];
  onSubmit: () => void;
  canProceed?: boolean;
  isPending: boolean;
  isEditMode?: boolean;
  onClose?: () => void;
  t: (key: string) => string;
  submitLabel?: string;
  setStep: (s: number) => void;
  onFooterChange?: (footer: ReactNode) => void;
  onSubHeaderChange?: (subHeader: ReactNode) => void;
}

/**
 * Manages the subheader (StepIndicator) and footer (Back/Next/Save buttons)
 * effects for multi-step form drawers. Call after declaring step state and mutation.
 */
export function useMultiStepFormEffects({
  step,
  totalSteps,
  stepLabels,
  onSubmit,
  canProceed = true,
  isPending,
  isEditMode = false,
  onClose,
  t,
  submitLabel,
  setStep,
  onFooterChange,
  onSubHeaderChange,
}: UseMultiStepFormOptions) {
  // Use refs for callbacks to avoid stale closures in effects
  const onSubmitRef = useRef(onSubmit);
  onSubmitRef.current = onSubmit;
  const setStepRef = useRef(setStep);
  setStepRef.current = setStep;
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Manage subheader (StepIndicator)
  useEffect(() => {
    if (isEditMode) {
      onSubHeaderChange?.(null);
    } else {
      onSubHeaderChange?.(<StepIndicator steps={stepLabels} current={step} />);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, isEditMode]);

  // Manage footer (Back/Next/Save buttons) when drawer-managed
  useEffect(() => {
    if (!onFooterChange) return;

    if (isEditMode) {
      onFooterChange(
        <div className="flex gap-2">
          <Button type="button" variant="secondary" size="sm" className="flex-1" onClick={() => onCloseRef.current?.()}>
            {t("common.cancel")}
          </Button>
          <Button size="sm" className="flex-1" onClick={() => onSubmitRef.current()} loading={isPending}>
            {t("common.save")}
          </Button>
        </div>
      );
      return;
    }

    onFooterChange(
      <div className="flex gap-2">
        {step > 1 && (
          <Button type="button" variant="secondary" size="sm" className="flex-1" onClick={() => setStepRef.current(step - 1)}>
            {t("common.back")}
          </Button>
        )}
        {step < totalSteps ? (
          <Button type="button" size="sm" className="flex-1" disabled={!canProceed} onClick={() => setStepRef.current(step + 1)}>
            {t("host.nextStep")}
          </Button>
        ) : (
          <Button size="sm" className="flex-1" onClick={() => onSubmitRef.current()} loading={isPending}>
            {submitLabel || t("common.create")}
          </Button>
        )}
      </div>
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [step, canProceed, isPending, isEditMode]);
}
