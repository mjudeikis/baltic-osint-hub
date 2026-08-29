// Shape encodes which layer a point belongs to; fill and size encode how much
// it matters within that layer.
//
// Everything used to be a circle in a different colour, with importance shown
// by dimming. That fails twice over: shade is a weak channel at 4–8px, and it
// carries no meaning at all for a colourblind reader or on a printed page. One
// visual channel per variable is the fix — a hollow diamond and a solid
// diamond are the same layer at a glance, and unmistakably different weights.

export type Shape = "circle" | "triangle" | "diamond" | "square" | "hex" | "line" | "area";

// Icon bitmaps are drawn at 4x and scaled down by MapLibre, so they stay crisp
// on high-density displays without shipping image assets.
const SCALE = 4;

function path(ctx: CanvasRenderingContext2D, shape: Shape, s: number) {
  const m = s * 0.14; // margin, so the stroke is not clipped by the bitmap edge
  const a = m;
  const b = s - m;
  const mid = s / 2;
  ctx.beginPath();
  switch (shape) {
    case "circle":
      ctx.arc(mid, mid, mid - m, 0, Math.PI * 2);
      break;
    case "triangle": // aircraft: points up
      ctx.moveTo(mid, a);
      ctx.lineTo(b, b);
      ctx.lineTo(a, b);
      ctx.closePath();
      break;
    case "diamond": // vessel
      ctx.moveTo(mid, a);
      ctx.lineTo(b, mid);
      ctx.lineTo(mid, b);
      ctx.lineTo(a, mid);
      ctx.closePath();
      break;
    case "hex": {
      const r = mid - m;
      for (let i = 0; i < 6; i++) {
        const ang = (Math.PI / 3) * i - Math.PI / 2;
        const x = mid + r * Math.cos(ang);
        const y = mid + r * Math.sin(ang);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.closePath();
      break;
    }
    case "square":
    case "line":
    case "area":
      ctx.rect(a, a, b - a, b - a);
      break;
  }
}

// makeIcon returns an ImageData bitmap for map.addImage().
//
// `filled` is the importance channel: a solid mark reads as something to look
// at, a hollow one as context. The outline is drawn in the surface colour so a
// mark stays legible when it overlaps another.
export function makeIcon(
  shape: Shape,
  color: string,
  filled: boolean,
  surface: string,
  px = 18,
): ImageData | null {
  const s = px * SCALE;
  const canvas = document.createElement("canvas");
  canvas.width = s;
  canvas.height = s;
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;

  if (filled) {
    path(ctx, shape, s);
    ctx.fillStyle = color;
    ctx.fill();
    ctx.lineWidth = 2 * SCALE;
    ctx.strokeStyle = surface;
    ctx.stroke();
  } else {
    // Hollow: the shape is still fully legible, but it does not claim
    // attention the way a filled mark does.
    path(ctx, shape, s);
    ctx.lineWidth = 2.5 * SCALE;
    ctx.strokeStyle = color;
    ctx.stroke();
  }
  return ctx.getImageData(0, 0, s, s);
}

// Swatch renders the same shape inline, so the layer toggles and the legend
// show exactly what appears on the map rather than a generic dot.
export function Swatch({
  shape,
  color,
  filled = true,
  size = 11,
}: {
  shape: Shape;
  color: string;
  filled?: boolean;
  size?: number;
}) {
  const fill = filled ? color : "none";
  const stroke = color;
  const sw = filled ? 0 : 2;
  const common = { fill, stroke, strokeWidth: sw };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 12 12"
      aria-hidden="true"
      style={{ flex: "none", verticalAlign: "middle" }}
    >
      {shape === "circle" && <circle cx="6" cy="6" r="5" {...common} />}
      {shape === "triangle" && <polygon points="6,1 11,11 1,11" {...common} />}
      {shape === "diamond" && <polygon points="6,1 11,6 6,11 1,6" {...common} />}
      {shape === "square" && <rect x="1" y="1" width="10" height="10" rx="1" {...common} />}
      {/* H3 interference cells really are hexagons on the map. */}
      {shape === "hex" && (
        <polygon points="6,0.5 10.8,3.25 10.8,8.75 6,11.5 1.2,8.75 1.2,3.25" {...common} />
      )}
      {/* Cables are drawn as a dashed line, so the key shows a dashed line. */}
      {shape === "line" && (
        <line x1="0.5" y1="6" x2="11.5" y2="6" stroke={stroke} strokeWidth="2" strokeDasharray="3 2" />
      )}
      {/* Territory is a tinted fill with an outline, not a marker. */}
      {shape === "area" && (
        <rect x="0.5" y="0.5" width="11" height="11" rx="1" fill={color} fillOpacity="0.25" stroke={stroke} strokeWidth="1" />
      )}
    </svg>
  );
}
