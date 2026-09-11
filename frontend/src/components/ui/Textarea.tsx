import { type TextareaHTMLAttributes, forwardRef, useId } from "react";
import FormField, { INPUT_CLASS, INPUT_ERROR_CLASS } from "./FormField";

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  hint?: string;
  error?: string;
}

const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(({ label, hint, error, required, className = "", ...props }, ref) => {
  const autoId = useId();
  const id = props.id ?? autoId;
  return (
    <FormField label={label} required={required} hint={hint} error={error} htmlFor={id}>
      <textarea ref={ref} id={id} required={required} aria-label={label ? undefined : props["aria-label"] ?? props.placeholder ?? props.title} className={`${INPUT_CLASS} resize-y ${error ? INPUT_ERROR_CLASS : ""} ${className}`} {...props} />
    </FormField>
  );
});

Textarea.displayName = "Textarea";
export default Textarea;
