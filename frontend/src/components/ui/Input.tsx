import { type InputHTMLAttributes, forwardRef } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  hint?: string;
  error?: string;
}

const Input = forwardRef<HTMLInputElement, InputProps>(({ label, hint, error, required, className = "", ...props }, ref) => (
  <FormField label={label} required={required} hint={hint} error={error}>
    <input ref={ref} required={required} className={`${INPUT_CLASS} ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props} />
  </FormField>
));

Input.displayName = "Input";
export default Input;
