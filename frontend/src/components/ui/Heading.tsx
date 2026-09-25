import type { ReactNode } from "react";

// Full literals: Tailwind only generates classes it can read in source.
const sizes = {
  xl: "text-heading-xl font-bold",
  lg: "text-heading-lg font-bold",
  md: "text-heading-md font-bold",
  sm: "text-heading-sm font-semibold",
  xs: "text-heading-xs font-semibold",
  xxs: "text-heading-xxs font-semibold",
} as const;

export type HeadingSize = keyof typeof sizes;

/**
 * A heading on the type scale (globals.css --text-heading-*). `as` is the
 * document level and `size` the look, so the outline stays correct (one h1
 * per page, no skipped levels) whatever size a spot needs.
 */
export default function Heading({
  as: Tag = "h2",
  size = "md",
  className = "",
  id,
  children,
}: {
  as?: "h1" | "h2" | "h3" | "h4";
  size?: HeadingSize;
  className?: string;
  id?: string;
  children: ReactNode;
}) {
  return (
    <Tag id={id} className={`font-display text-[var(--text-primary)] ${sizes[size]} ${className}`}>
      {children}
    </Tag>
  );
}
