export function safeReturnTo(state: unknown, fallback = "/home"): string {
  if (!state || typeof state !== "object") return fallback;
  const returnTo = (state as { returnTo?: unknown }).returnTo;
  if (
    typeof returnTo !== "string" ||
    !returnTo.startsWith("/") ||
    returnTo.startsWith("//") ||
    returnTo.startsWith("/\\") ||
    returnTo === "/login" ||
    returnTo === "/register"
  ) {
    return fallback;
  }
  return returnTo;
}
