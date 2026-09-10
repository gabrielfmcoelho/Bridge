// Stroke icon from a 24x24 `d` path (lib/icon-paths). `size` scales the stroke
// with the box so small glyphs don't read heavier than large ones; `className`
// and `strokeWidth` still override for the odd one-off.
const sizes = {
  xs: { box: "w-3.5 h-3.5", stroke: 2 },
  sm: { box: "w-4 h-4", stroke: 2 },
  md: { box: "w-5 h-5", stroke: 1.75 },
  lg: { box: "w-6 h-6", stroke: 1.5 },
};

interface IconProps {
  path: string;
  size?: keyof typeof sizes;
  className?: string;
  strokeWidth?: number;
}

export default function Icon({ path, size, className, strokeWidth }: IconProps) {
  const preset = sizes[size ?? "sm"];
  return (
    <svg
      className={`shrink-0 ${className ?? preset.box}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={strokeWidth ?? preset.stroke}
      aria-hidden
    >
      <path strokeLinecap="round" strokeLinejoin="round" d={path} />
    </svg>
  );
}
