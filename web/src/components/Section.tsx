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
