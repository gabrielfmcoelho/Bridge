import type { ReactNode } from "react";
import SectionHeading from "@/components/ui/SectionHeading";

/** One section of an EntityFormShell form: a heading on a hairline, an
 *  optional one-line purpose, then its fields (two columns from sm up unless
 *  `stack`). The id is what FormSectionNav scrolls to. */
export default function FormSection({ id, title, description, stack = false, children }: {
  id: string;
  title: string;
  description?: string;
  /** One column: lists and editors that want the full width. */
  stack?: boolean;
  children: ReactNode;
}) {
  return (
    <section id={id} aria-labelledby={`${id}-title`} className="scroll-mt-3">
      <SectionHeading as="h3" variant="rule" hint={description}>
        <span id={`${id}-title`}>{title}</span>
      </SectionHeading>
      <div className={stack ? "space-y-4" : "grid grid-cols-1 sm:grid-cols-2 gap-x-4 gap-y-4"}>
        {children}
      </div>
    </section>
  );
}
