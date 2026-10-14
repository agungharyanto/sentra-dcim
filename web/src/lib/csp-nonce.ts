import { setNonce } from "get-nonce";

/**
 * Hands the per-request CSP nonce (ISS-030, D-065) to react-style-singleton, the library behind Radix's scroll lock
 * and the only runtime <style> injector in the app. Caddy writes the nonce into <meta name="csp-nonce"> of
 * index.html and into the style-src of the same response; without it (Vite dev server) nothing is set.
 */
export function applyCspNonce(doc: Document = document): string | undefined {
  const nonce = doc.querySelector<HTMLMetaElement>('meta[name="csp-nonce"]')?.content ?? "";
  // The unfilled template text ({{…}}) means: not served through Caddy.
  if (!/^[A-Za-z0-9+/_=-]{16,}$/.test(nonce)) return undefined;
  setNonce(nonce);
  return nonce;
}
