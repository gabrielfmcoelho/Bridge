"use client";

// Scalar's React wrapper imports this stylesheet internally, but that
// side-effect import lives inside the ssr:false dynamic chunk below — which
// Next/Turbopack does NOT reliably inject, leaving the reference fully
// unstyled. Importing it statically here (this file is imported directly by
// the page) puts it in the route's CSS bundle, guaranteeing it loads.
import "@scalar/api-reference-react/style.css";
import dynamic from "next/dynamic";
import { useTheme } from "@/contexts/ThemeContext";
import { useLocale } from "@/contexts/LocaleContext";

const ApiReferenceReact = dynamic(
  () => import("@scalar/api-reference-react").then((m) => ({ default: m.ApiReferenceReact })),
  {
    ssr: false,
    loading: () => <div className="p-6 text-sm text-[var(--text-muted)]">Loading API reference…</div>,
  }
);

// customCss strips Scalar's own page background so the reference blends into
// the app surface. We zero out the layout containers AND the --scalar-background-1
// CSS variable (Scalar's main canvas/sidebar background); backgrounds 2–4 are
// left intact so code samples, cards, and the test-request panel keep enough
// contrast to stay readable. Theme is forced to match the active app theme.
const SCALAR_CSS = `
.scalar-app,
.scalar-api-reference,
.scalar-app-layout,
.references-layout { background: transparent !important; }
.scalar-api-reference,
.scalar-app {
  --scalar-background-1: transparent !important;
  --scalar-sidebar-background-1: transparent !important;
}
/* The Test Request / API-client modal inherits the transparent canvas above,
   which leaves it see-through over the endpoint list. Restore Scalar's solid
   background for the modal subtree based on the active theme. */
[data-theme="dark"] .scalar-client--open,
[data-theme="dark"] .scalar-modal,
[data-theme="dark"] .scalar-modal-layout,
[data-theme="dark"] .scalar-modal-layout-full,
[data-theme="dark"] .scalar-modal-body {
  --scalar-background-1: #0f0f0f !important;
  --scalar-sidebar-background-1: #0f0f0f !important;
  background-color: #0f0f0f !important;
}
[data-theme="light"] .scalar-client--open,
[data-theme="light"] .scalar-modal,
[data-theme="light"] .scalar-modal-layout,
[data-theme="light"] .scalar-modal-layout-full,
[data-theme="light"] .scalar-modal-body {
  --scalar-background-1: #ffffff !important;
  --scalar-sidebar-background-1: #ffffff !important;
  background-color: #ffffff !important;
}
`;

// ApiReference renders a full OpenAPI/Swagger document. content is the
// already-fetched spec object (fetched through our authenticated client).
// The API-client launcher and Ask-AI assistant are always disabled. The
// sidebar and per-endpoint "Test Request" affordance are configurable: the
// per-endpoint page keeps both; the public share page turns both off for a leaner,
// read-only reference.
export default function ApiReference({
  content,
  showSidebar = true,
  hideTestRequest = false,
  serverUrl,
}: {
  content: Record<string, unknown>;
  showSidebar?: boolean;
  hideTestRequest?: boolean;
  // When set, overrides the spec's servers so Scalar's "Test Request" targets
  // the real API host instead of falling back to this app's origin.
  serverUrl?: string;
}) {
  const { theme } = useTheme();
  const { locale } = useLocale();
  const isPt = locale === "pt-BR";

  const customCss = `
${SCALAR_CSS}
.show-more,
.group\\/summary button {
  font-size: 0 !important;
}
.show-more::after {
  content: "${isPt ? "Mostrar mais" : "Show More"}" !important;
  font-size: var(--scalar-font-size-3, 13px) !important;
}
.group\\/summary button::after,
.group-summary button::after {
  content: "${isPt ? "Mais" : "More"}" !important;
  font-size: var(--scalar-font-size-3, 13px) !important;
}
.group\\/summary button[aria-expanded="true"]::after,
.group-summary button[aria-expanded="true"]::after {
  content: "${isPt ? "Mostrar menos" : "Show Less"}" !important;
  font-size: var(--scalar-font-size-3, 13px) !important;
}
`;

  const doc = serverUrl ? { ...content, servers: [{ url: serverUrl }] } : content;
  return (
    <ApiReferenceReact
      key={`${theme}-${locale}`}
      configuration={{
        content: doc,
        showSidebar,
        hideClientButton: true,
        hideTestRequestButton: hideTestRequest,
        hideDownloadButton: true,
        hideDarkModeToggle: true,
        forceDarkModeState: theme,
        showDeveloperTools: "never",
        agent: { disabled: true },
        mcp: { disabled: true },
        customCss,
      }}
    />
  );
}
