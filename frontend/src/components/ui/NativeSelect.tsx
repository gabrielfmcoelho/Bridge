import { type SelectHTMLAttributes, forwardRef } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface NativeSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  hint?: string;
  error?: string;
}

// Plain <select> in the Input skin, for short fixed option lists and forms
// that want a real change event. Select (Radix) is the searchable one.
const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ label, hint, error, required, className = "", children, ...props }, ref) => (
    <FormField label={label} required={required} hint={hint} error={error}>
      <select ref={ref} required={required} className={`${INPUT_CLASS} ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props}>
        {children}
      </select>
    </FormField>
  )
);

NativeSelect.displayName = "NativeSelect";
export default NativeSelect;
