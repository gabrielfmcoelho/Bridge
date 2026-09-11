import { type SelectHTMLAttributes, forwardRef, useId } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface NativeSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  hint?: string;
  error?: string;
}

// Plain <select> in the Input skin, for short fixed option lists and forms
// that want a real change event. Select (Radix) is the searchable one.
const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ label, hint, error, required, className = "", children, ...props }, ref) => {
    const autoId = useId();
    const id = props.id ?? autoId;
    return (
      <FormField label={label} required={required} hint={hint} error={error} htmlFor={id}>
        <select ref={ref} id={id} required={required} className={`${INPUT_CLASS} ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props}>
          {children}
        </select>
      </FormField>
    );
  }
);

NativeSelect.displayName = "NativeSelect";
export default NativeSelect;
