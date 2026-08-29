import { useEffect, useState } from "react";

export interface NavItem {
  id: string;
  label: string;
}

// Sticky section navigation. The active item follows the scroll position via
// IntersectionObserver rather than scroll maths, so it stays correct when
// sections are collapsed and the page height changes.
export default function SideNav({ items }: { items: NavItem[] }) {
  const [active, setActive] = useState(items[0]?.id ?? "");

  useEffect(() => {
    const visible = new Set<string>();
    const observer = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) visible.add(e.target.id);
          else visible.delete(e.target.id);
        }
        // Highest section currently on screen wins.
        const first = items.find((i) => visible.has(i.id));
        if (first) setActive(first.id);
      },
      { rootMargin: "-80px 0px -60% 0px" },
    );
    for (const i of items) {
      const el = document.getElementById(i.id);
      if (el) observer.observe(el);
    }
    return () => observer.disconnect();
  }, [items]);

  return (
    <nav className="sidenav" aria-label="Sections">
      <ul>
        {items.map((i) => (
          <li key={i.id}>
            {/* Plain anchors: native hash navigation works even when the
                browser throttles scripted scrolling (background tabs), and
                CSS scroll-behavior handles the smoothness. */}
            <a href={`#${i.id}`} aria-current={active === i.id ? "true" : undefined}>
              {i.label}
            </a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
