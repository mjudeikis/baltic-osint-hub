import { ReactNode, useEffect, useState } from "react";

// Collapsible section. Open/closed state persists per section so a reader's
// layout survives a reload; storage failures (private windows, blocked site
// data) fall back to "open" rather than breaking the page.
function readStored(id: string, fallback: boolean): boolean {
  try {
    const v = localStorage.getItem(`section:${id}`);
    return v === null ? fallback : v === "open";
  } catch {
    return fallback;
  }
}

// revealSection opens a (possibly collapsed) section and scrolls to it.
// Every drill-through must use this rather than scrollIntoView alone: a reader
// who collapsed the feed and then clicks a board tile would otherwise land on
// a closed header and the click would look dead.
export function revealSection(id: string) {
  window.dispatchEvent(new CustomEvent(OPEN_EVENT, { detail: id }));
  // Next frame, so an opening section lays out before the scroll measures it.
  requestAnimationFrame(() =>
    document.getElementById(id)?.scrollIntoView({ block: "start" }),
  );
}

const OPEN_EVENT = "section:open";

export default function Section({
  id,
  title,
  aside,
  defaultOpen = true,
  children,
}: {
  id: string;
  title: string;
  aside?: ReactNode;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(() => readStored(id, defaultOpen));

  // Open when navigated to: the sidenav's hash links and revealSection both
  // target sections that may be collapsed.
  useEffect(() => {
    const onHash = () => {
      if (location.hash === `#${id}`) setOpen(true);
    };
    const onOpen = (e: Event) => {
      if ((e as CustomEvent<string>).detail === id) setOpen(true);
    };
    onHash();
    window.addEventListener("hashchange", onHash);
    window.addEventListener(OPEN_EVENT, onOpen);
    return () => {
      window.removeEventListener("hashchange", onHash);
      window.removeEventListener(OPEN_EVENT, onOpen);
    };
  }, [id]);

  useEffect(() => {
    try {
      localStorage.setItem(`section:${id}`, open ? "open" : "closed");
    } catch {
      /* storage unavailable — state stays for this session only */
    }
    // A canvas map sized while hidden renders at zero; nudge it once the
    // section is visible again.
    if (open) {
      requestAnimationFrame(() => window.dispatchEvent(new Event("resize")));
    }
  }, [id, open]);

  return (
    <section className="card section" id={id} aria-label={title}>
      <div className="section-head">
        <button
          className="section-toggle"
          aria-expanded={open}
          aria-controls={`${id}-body`}
          onClick={() => setOpen((v) => !v)}
        >
          <span className="chevron" data-open={open} aria-hidden="true">
            ▸
          </span>
          <h2>{title}</h2>
        </button>
        {aside && <div className="section-aside">{aside}</div>}
      </div>
      <div id={`${id}-body`} hidden={!open}>
        {children}
      </div>
    </section>
  );
}
