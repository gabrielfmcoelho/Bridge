// Descaracterização eleitoral: during an election blackout the public share
// page must not display state branding. Set DESCARACTERIZACAO_ELEITORAL=true as
// a RUNTIME env var (docker run -e ... / next start with the var) — no rebuild
// needed. Read server-side here and flagged onto a wrapper the CSS keys off
// (.piaui-backdrop, see globals.css). force-dynamic guarantees the env is read
// per request instead of being baked into a static prerender at build time.
export const dynamic = "force-dynamic";

export default function ShareLayout({ children }: { children: React.ReactNode }) {
  const descaracterizacao = process.env.DESCARACTERIZACAO_ELEITORAL === "true";
  return (
    <div data-descaracterizacao={descaracterizacao ? "true" : undefined}>
      {children}
    </div>
  );
}
