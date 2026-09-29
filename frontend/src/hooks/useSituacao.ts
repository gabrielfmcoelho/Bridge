import { useQuery } from "@tanstack/react-query";
import { enumsAPI } from "@/lib/api";
import { situacaoRole } from "@/lib/constants";

/** The situação options with lookups by value and by role, so code asks for
 *  "the active one" instead of comparing to a value an admin can rename. */
export function useSituacao() {
  const { data: options = [] } = useQuery({ queryKey: ["enums", "situacao"], queryFn: () => enumsAPI.list("situacao") });
  return {
    options,
    roleOf: (value: string | undefined) => situacaoRole(value, options),
    colorOf: (value: string | undefined) => options.find((o) => o.value === value)?.color,
    /** The value that carries a role; the role name itself until the options load or when none carries it. */
    valueOf: (role: string) => options.find((o) => o.role === role)?.value ?? role,
  };
}
