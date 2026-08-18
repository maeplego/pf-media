import { NextRequest, NextResponse } from "next/server";

import { clearOn, readRequestCookie, setOn } from "../../lib/oidc/cookies";
import { clientId, internalBase, oidcEnabled, publicOrigin, redirectUri } from "../../lib/oidc/env";
import { verifyIdToken } from "../../lib/oidc/idtoken";

export async function GET(req: NextRequest) {
  const origin = publicOrigin(req);
  if (!oidcEnabled()) {
    return NextResponse.redirect(new URL("/", origin));
  }
  const url = req.nextUrl;
  const err = url.searchParams.get("error");
  if (err) {
    return NextResponse.redirect(new URL(`/?error=${encodeURIComponent(err)}`, origin));
  }
  const code = url.searchParams.get("code") ?? "";
  const state = url.searchParams.get("state") ?? "";
  const expected = readRequestCookie(req, "rp_state");
  const nonce = readRequestCookie(req, "rp_nonce");
  const verifier = readRequestCookie(req, "rp_verifier");
  if (!code || !state || !expected || state !== expected || !nonce || !verifier) {
    return NextResponse.redirect(new URL("/?error=state", origin));
  }

  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: clientId(),
    code,
    redirect_uri: redirectUri(),
    code_verifier: verifier,
  });
  const tokenRes = await fetch(`${internalBase()}/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
    cache: "no-store",
  });
  if (!tokenRes.ok) {
    return NextResponse.redirect(new URL("/?error=token", origin));
  }
  const tokens = (await tokenRes.json()) as {
    access_token?: string;
    id_token?: string;
    refresh_token?: string;
  };
  if (!tokens.access_token || !tokens.id_token) {
    return NextResponse.redirect(new URL("/?error=token", origin));
  }
  try {
    await verifyIdToken(tokens.id_token, nonce);
    const res = NextResponse.redirect(new URL("/", origin));
    setOn(res, "rp_access", tokens.access_token);
    setOn(res, "rp_id", tokens.id_token);
    if (tokens.refresh_token) {
      setOn(res, "rp_refresh", tokens.refresh_token);
    }
    clearOn(res, "rp_state");
    clearOn(res, "rp_nonce");
    clearOn(res, "rp_verifier");
    return res;
  } catch {
    return NextResponse.redirect(new URL("/?error=id_token", origin));
  }
}
