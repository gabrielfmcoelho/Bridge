import { useEffect, useRef } from "react";

const eventName = (param: string) => `open-param:${param}`;

/**
 * Opens something from outside the page: `open` runs once on mount when
 * `?{param}` is in the URL (the header's "Create" menu links to /hosts?new=1),
 * then the param is dropped. Reads window.location instead of useSearchParams,
 * which would force a Suspense boundary at build time.
 *
 * A soft navigation to the same path doesn't remount the page, so callers
 * already on it use `requestOpen(param)` instead of navigating.
 *
 * `enabled` (e.g. canEdit) waits for the user to load and gates the action:
 * a viewer following a ?new=1 link just sees the list.
 */
export function useOpenOnParam(param: string, open: () => void, enabled = true) {
  const openRef = useRef(open);
  useEffect(() => { openRef.current = open; });
  useEffect(() => {
    if (!enabled) return;
    const url = new URL(window.location.href);
    if (url.searchParams.has(param)) {
      openRef.current();
      url.searchParams.delete(param);
      // history.replaceState, not router.replace: no re-render/refetch; the app
      // router stays in sync with native history.
      window.history.replaceState(window.history.state, "", url.pathname + url.search + url.hash);
    }
    const onRequest = () => openRef.current();
    window.addEventListener(eventName(param), onRequest);
    return () => window.removeEventListener(eventName(param), onRequest);
  }, [param, enabled]);
}

export function requestOpen(param: string) {
  window.dispatchEvent(new Event(eventName(param)));
}
