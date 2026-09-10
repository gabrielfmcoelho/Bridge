import { type TextareaHTMLAttributes, forwardRef } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  hint?: string;
  error?: string;
}

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(({ label, hint, error, required, className = "", ...props }, ref) => (
  <FormField label={label} required={required} hint={hint} error={error}>
    <textarea ref={ref} required={required} className={`${INPUT_CLASS} resize-y ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props} />
  </FormField>
));

Textarea.displayName = "Textarea";
export default Textarea;
