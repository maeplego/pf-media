export function oidcEnabled(): boolean {
  return !!(process.env.OIDC_ISSUER?.trim() && process.env.OIDC_CLIENT_ID?.trim());
}

export function issuer(): string {
  const v = process.env.OIDC_ISSUER?.replace(/\/$/, "");
  if (!v) {
    throw new Error("OIDC_ISSUER is required");
  }
  return v;
}

export function internalBase(): string {
  const v = process.env.OIDC_INTERNAL_BASE?.replace(/\/$/, "");
  return v || issuer();
}

export function clientId(): string {
  const v = process.env.OIDC_CLIENT_ID;
  if (!v) {
    throw new Error("OIDC_CLIENT_ID is required");
  }
  return v;
}

export function redirectUri(): string {
  const v = process.env.OIDC_REDIRECT_URI;
  if (!v) {
    throw new Error("OIDC_REDIRECT_URI is required");
  }
  return v;
}

export function postLogoutRedirectUri(): string {
  const v = process.env.OIDC_POST_LOGOUT_REDIRECT_URI;
  if (v) {
    return v;
  }
  return redirectUri().replace(/\/callback$/, "/logged-out");
}

// Browser-facing origin. K8s sets HOSTNAME to the pod name, so req.nextUrl.origin is unusable.
export function publicOrigin(req: { headers: Headers; nextUrl: URL; url: string }): string {
  const redirect = process.env.OIDC_REDIRECT_URI?.trim();
  if (redirect) {
    try {
      return new URL(redirect).origin;
    } catch {
      // fall through to forwarded headers
    }
  }
  const host = req.headers.get("x-forwarded-host") || req.headers.get("host");
  if (host) {
    const proto = req.headers.get("x-forwarded-proto") || "http";
    return `${proto.split(",")[0].trim()}://${host.split(",")[0].trim()}`;
  }
  return req.nextUrl.origin;
}
