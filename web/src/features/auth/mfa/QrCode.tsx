/**
 * QR code drawn as SVG from the server's module matrix (rows of "1"/"0"), with the 4-module quiet zone scanners
 * need. Black on white in both themes (dark-mode inverted QR codes are not read by every authenticator app). No
 * image data URL: nothing to relax in the CSP.
 */
export function QrCode({ rows, label, size = 240 }: { rows: string[]; label: string; size?: number }) {
  const n = rows.length + 8;
  let d = "";
  rows.forEach((row, y) => {
    for (let x = 0; x < row.length; x++) if (row[x] === "1") d += `M${x + 4} ${y + 4}h1v1h-1z`;
  });
  return (
    <svg
      role="img"
      aria-label={label}
      width={size}
      height={size}
      viewBox={`0 0 ${n} ${n}`}
      shapeRendering="crispEdges"
      className="rounded-md border"
      data-testid="mfa-qr"
    >
      <rect width={n} height={n} fill="#ffffff" />
      <path d={d} fill="#000000" />
    </svg>
  );
}
