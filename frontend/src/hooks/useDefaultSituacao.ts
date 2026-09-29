import { useEffect } from "react";
import { useSituacao } from "@/hooks/useSituacao";

/**
 * A new record starts in the situação that carries the "active" role. The
 * options load after the form mounts, so until then the value may be the
 * bare role name; once they arrive, a value that isn't an option is swapped
 * for the active one. Create forms only: an edited record keeps whatever it
 * has, even a value no option carries any more.
 */
export function useDefaultSituacao(value: string, set: (v: string) => void, enabled: boolean) {
  const { options, valueOf } = useSituacao();
  useEffect(() => {
    if (enabled && options.length && !options.some((o) => o.value === value)) set(valueOf("active"));
    // eslint-disable-next-line react-hooks/exhaustive-deps -- runs when the options arrive or the value changes
  }, [options, value]);
}
