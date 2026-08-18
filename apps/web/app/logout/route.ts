import { NextRequest, NextResponse } from "next/server";

import { readCookie, setOn } from "../../lib/oidc/cookies";
import { clientId, issuer, oidcEnabled, postLogoutRedirectUri } from "../../lib/oidc/env";
import { randomString } from "../../lib/oidc/pkce";

export async function POST(req: NextRequest) {
  if (!oidcEnabled()) {
    return NextResponse.redirect(new URL("/", req.url), { status: 303 });
  }
  const idToken = await readCookie("rp_id");
  const state = randomString(16);
  const q = new URLSearchParams({
    client_id: clientId(),
    post_logout_redirect_uri: postLogoutRedirectUri(),
    state,
  });
  if (idToken) {
    q.set("id_token_hint", idToken);
  }
  const res = NextResponse.redirect(`${issuer()}/end-session?${q.toString()}`, { status: 303 });
  setOn(res, "rp_logout_state", state, 600);
  return res;
}
