interface IconProps {
  path: string;
  className?: string;
  strokeWidth?: number;
}

export default function Icon({ path, className = "w-4 h-4", strokeWidth = 2 }: IconProps) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={strokeWidth}>
      <path strokeLinecap="round" strokeLinejoin="round" d={path} />
    </svg>
  );
}
