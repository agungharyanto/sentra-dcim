/** Title key (auth namespace) of the own-account pages inside the shell, or undefined. */
export function accountPageTitleKey(pathname: string): "sessions.title" | "password.title" | "security.title" | undefined {
  if (pathname === "/account/sessions") return "sessions.title";
  if (pathname === "/account/password") return "password.title";
  if (pathname === "/account/security") return "security.title";
  return undefined;
}
