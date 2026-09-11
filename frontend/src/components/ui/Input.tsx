import { type InputHTMLAttributes, forwardRef, useId } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: string;
  error?: string;
}

const Input = forwardRef<HTMLInputElement, InputProps>(({ label, hint, error, required, className = "", ...props }, ref) => {
  const autoId = useId();
  const id = props.id ?? autoId;
  return (
    <FormField label={label} required={required} hint={hint} error={error} htmlFor={id}>
      <input ref={ref} id={id} required={required} aria-label={label ? undefined : props["aria-label"] ?? props.placeholder ?? props.title} className={`${INPUT_CLASS} ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props} />
    </FormField>
  );
});

Input.displayName = "Input";
export default Input;
