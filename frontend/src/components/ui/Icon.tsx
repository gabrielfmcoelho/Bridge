// Stroke icon from a 24x24 `d` path (lib/icon-paths). `size` scales the stroke
// with the box so small glyphs don't read heavier than large ones. `className`
// adds to the preset; a className with its own w-/h-/size- replaces the box.
const sizes = {
  xs: { box: "w-3.5 h-3.5", stroke: 2 },
  sm: { box: "w-4 h-4", stroke: 2 },
  md: { box: "w-5 h-5", stroke: 1.75 },
  lg: { box: "w-6 h-6", stroke: 1.5 },
};

interface IconProps extends Omit<React.SVGProps<SVGSVGElement>, "path" | "strokeWidth"> {
  path: string;
  size?: keyof typeof sizes;
  className?: string;
  strokeWidth?: number;
}

export default function Icon({ path, size, className, strokeWidth, ...rest }: IconProps) {
  const preset = sizes[size ?? "sm"];
  return (
    <svg
      // Merge, don't replace: a colour-only className must keep the preset box
      // (replacing it once made a delete icon fill its whole button).
      className={`shrink-0 ${className && /(^|[\s:])(w|h|size)-/.test(className) ? className : `${preset.box} ${className ?? ""}`}`}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={strokeWidth ?? preset.stroke}
      aria-hidden
      {...rest}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d={path} />
    </svg>
  );
}
