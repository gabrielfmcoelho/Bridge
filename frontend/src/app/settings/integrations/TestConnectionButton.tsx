"use client";

import { useMutation } from "@tanstack/react-query";
import { useLocale } from "@/contexts/LocaleContext";
import Button from "@/components/ui/Button";
import StatusAlert from "@/components/ui/StatusAlert";

export interface TestResult {
  success: boolean;
  message: string;
}

/**
 * "Test connection" and its result. `run` calls the section's test endpoint
 * and turns the response into one line; a thrown error is shown as a failure.
 */
export default function TestConnectionButton({ run, disabled }: { run: () => Promise<TestResult>; disabled?: boolean }) {
  const { t } = useLocale();
  const test = useMutation({ mutationFn: run });
  const result: TestResult | null = test.isError
    ? { success: false, message: test.error.message || t("settings.integrations.testFailed") }
    : (test.data ?? null);

  return (
    <div className="space-y-3">
      <Button type="button" variant="secondary" onClick={() => test.mutate()} loading={test.isPending} disabled={disabled}>
        {t("settings.integrations.testConnection")}
      </Button>
      {result && !test.isPending && (
        <StatusAlert variant={result.success ? "success" : "error"}>{result.message}</StatusAlert>
      )}
    </div>
  );
}

/** The failure line most test endpoints share: "<stage>: <error>". */
export function failedLine(t: (key: string, vars?: Record<string, string>) => string, error?: string, stage?: string) {
  return t("settings.integrations.stageFailedDetail", {
    stage: stage ?? t("settings.integrations.connectionFailed"),
    error: error || t("settings.integrations.unknownError"),
  });
}
