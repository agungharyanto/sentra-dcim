import idErrors from "./locales/id/errors.json";

export type ErrorCode = keyof typeof idErrors.code;

/** Maps an API error code to a translatable key (errors:code.<code>); unknown codes fall back to "unknown". */
export function toErrorCode(code: string): ErrorCode {
  return Object.hasOwn(idErrors.code, code) ? (code as ErrorCode) : "unknown";
}
