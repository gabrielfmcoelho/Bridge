import type { CSSProperties, ReactNode } from "react";
import Icon from "./Icon";
import { ICON_PATHS } from "@/lib/icon-paths";

// Named accents resolve to theme tokens; anything else passes through as a CSS
// colour (the situacao enum ships hex from the backend). The hue keys are the
// legacy names existing call sites still pass.
const ACCENTS: Record<string, string> = {
  accent: "var(--accent)",
  success: "var(--success)",
  warning: "var(--warning)",
  danger: "var(--danger)",
  info: "var(--info)",
  cyan: "var(--cyan)",
  purple: "var(--accent)", // alias: --purple was removed
  rose: "var(--rose)",
  muted: "var(--text-faint)",
  emerald: "var(--success)",
  amber: "var(--warning)",
  red: "var(--danger)",
  sky: "var(--info)",
};

export type CardAccent = keyof typeof ACCENTS | "none" | (string & {});

export function accentColor(accent?: CardAccent): string | undefined {
  if (!accent || accent === "none") return undefined;
  return ACCENTS[accent] ?? accent;
}

const paddings = {
  md: "p-3.5 md:p-5",
  sm: "p-4",
  none: "",
};

interface CardProps {
  children: ReactNode;
  className?: string;
  id?: string;
  style?: CSSProperties;
  onClick?: () => void;
  /** Token name (`success`, `cyan`, …) or any CSS colour. Exposed to children as `var(--card-accent)`. */
  accent?: CardAccent;
  /** How the accent shows. Defaults to `stripe-left` when an accent is given, `none` otherwise. */
  decorator?: "stripe-left" | "stripe-top" | "tint" | "none";
  padding?: keyof typeof paddings;
  hover?: boolean;
  selected?: boolean;
  clickIndicator?: "link" | "drawer";
  /** `button` makes the whole card the control (catalog, atlas). Links wrap the card instead. */
  as?: "div" | "button";
}

const indicatorIcons: Record<string, string> = {
  link: ICON_PATHS.chevronRight,
  drawer: "M4 6h16M4 12h16M4 18h7",
};

export default function Card({
  children,
  className = "",
  id,
  style,
  onClick,
  accent,
  decorator,
  padding = "md",
  hover = true,
  selected = false,
  clickIndicator,
  as: Tag = "div",
}: CardProps) {
  const color = accentColor(accent);
  const deco = decorator ?? (color ? "stripe-left" : "none");
  const clickable = Tag === "button" || !!onClick;

  const decoClass = {
    "stripe-left": "border-l-[3px] border-l-[var(--card-accent)]",
    "stripe-top": "border-t-[3px] border-t-[var(--card-accent)]",
    // dark: a tinted wash. light: the same gradient turns into a pastel
    // rectangle, so the accent becomes a top rule over a flat surface instead.
    tint: "overflow-hidden border-[var(--card-accent)]/20 light:border-t-[3px] light:border-t-[var(--card-accent)] light:border-[var(--border-subtle)]",
    none: "",
  }[deco];

  // No lift: a grid of operational rows that bounces under the pointer is
  // toy-like, and the shadow that was supposed to explain the movement is
  // invisible on a near-black ground. Hover changes the surface instead.
  const hoverClass = hover
    ? `hover:bg-[var(--bg-elevated)] ${deco === "tint" ? "" : "hover:border-[var(--border-strong)]"}`
    : "";

  const borderClass = selected
    ? "border-[var(--accent)] ring-2 ring-[var(--accent)]/30"
    : deco === "tint"
      ? ""
      : "border-[var(--border-subtle)]";

  return (
    <Tag
      id={id}
      type={Tag === "button" ? "button" : undefined}
      className={`
        group/card relative
        bg-[var(--bg-surface)] rounded-[var(--radius-lg)] border ${borderClass} ${paddings[padding]}
        shadow-[var(--elevate)]
        transition duration-150 ease-out
        ${hoverClass}
        ${clickable ? "cursor-pointer active:scale-[0.99]" : ""}
        ${Tag === "button" ? "w-full text-left" : ""}
        ${decoClass}
        ${className}
      `}
      style={{ ...style, "--card-accent": color ?? "var(--accent)" } as CSSProperties}
      onClick={onClick}
    >
      {deco === "tint" && (
        <div className="absolute inset-0 bg-gradient-to-br from-[var(--card-accent)]/10 to-transparent pointer-events-none light:hidden" />
      )}
      {clickIndicator && (
        <Icon
          path={indicatorIcons[clickIndicator]}
          className="absolute bottom-2 right-2.5 w-3.5 h-3.5 text-[var(--text-faint)] opacity-80 group-hover/card:opacity-100 transition-opacity"
        />
      )}
      {children}
    </Tag>
  );
}

/** Icon tile for a card header; takes the card's accent on hover. Compose it inside `Card` like the inventory parts. */
export function CardIcon({ path, className = "" }: { path: string; className?: string }) {
  return (
    <span
      className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-[var(--radius-md)] border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)] transition-colors duration-150 group-hover/card:border-[var(--card-accent)] group-hover/card:bg-[var(--card-accent)]/10 group-hover/card:text-[var(--card-accent)] ${className}`}
    >
      <Icon path={path} className="h-5 w-5" strokeWidth={1.5} />
    </span>
  );
}
