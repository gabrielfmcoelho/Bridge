"use client";

// Scalar's React wrapper imports this stylesheet internally, but that
// side-effect import lives inside the ssr:false dynamic chunk below — which
// Next/Turbopack does NOT reliably inject, leaving the reference fully
// unstyled. Importing it statically here (this file is imported directly by
// the page) puts it in the route's CSS bundle, guaranteeing it loads.
import "@scalar/api-reference-react/style.css";
import dynamic from "next/dynamic";

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
// contrast to stay readable. Dark mode is forced to match the dark app shell.
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
   dark background (its default --scalar-background-1) for the modal subtree. */
.scalar-client--open,
.scalar-modal,
.scalar-modal-layout,
.scalar-modal-layout-full,
.scalar-modal-body {
  --scalar-background-1: #0f0f0f !important;
  --scalar-sidebar-background-1: #0f0f0f !important;
  background-color: #0f0f0f !important;
}
`;

// ApiReference renders a full OpenAPI/Swagger document. content is the
// already-fetched spec object (fetched through our authenticated client).
// The API-client launcher and Ask-AI assistant are always disabled. The
// sidebar and per-endpoint "Test Request" affordance are configurable: the
// detail page keeps both; the public share page turns both off for a leaner,
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
  const doc = serverUrl ? { ...content, servers: [{ url: serverUrl }] } : content;
  return (
    <ApiReferenceReact
      configuration={{
        content: doc,
        showSidebar,
        hideClientButton: true,
        hideTestRequestButton: hideTestRequest,
        hideDownloadButton: true,
        hideDarkModeToggle: true,
        forceDarkModeState: "dark",
        agent: { disabled: true },
        mcp: { disabled: true },
        customCss: SCALAR_CSS,
      }}
    />
  );
}
