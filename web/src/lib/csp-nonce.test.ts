import { getNonce } from "get-nonce";

import { applyCspNonce } from "./csp-nonce";

function docWith(content: string | null): Document {
  const d = document.implementation.createHTMLDocument();
  if (content !== null) {
    const m = d.createElement("meta");
    m.name = "csp-nonce";
    m.content = content;
    d.head.append(m);
  }
  return d;
}

describe("applyCspNonce", () => {
  it("hands the Caddy nonce to react-style-singleton", () => {
    expect(applyCspNonce(docWith("6a1f0c2e-8b7d-4c3a-9e5f-0123456789ab"))).toBe("6a1f0c2e-8b7d-4c3a-9e5f-0123456789ab");
    expect(getNonce()).toBe("6a1f0c2e-8b7d-4c3a-9e5f-0123456789ab");
  });

  it("ignores the unfilled template and a missing meta (Vite dev server)", () => {
    expect(applyCspNonce(docWith('{{placeholder "http.request.uuid"}}'))).toBeUndefined();
    expect(applyCspNonce(docWith(null))).toBeUndefined();
  });
});
