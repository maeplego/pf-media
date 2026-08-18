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
