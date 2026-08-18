import { NextRequest, NextResponse } from "next/server";

import { clearOn, readRequestCookie } from "../../lib/oidc/cookies";
import { oidcEnabled } from "../../lib/oidc/env";

export async function GET(req: NextRequest) {
  if (!oidcEnabled()) {
    return NextResponse.redirect(new URL("/", req.url), { status: 303 });
  }
  const state = req.nextUrl.searchParams.get("state") ?? "";
  const expected = readRequestCookie(req, "rp_logout_state") ?? "";
  if (!expected || state !== expected) {
    const res = NextResponse.redirect(new URL("/?error=logout_state", req.url), { status: 303 });
    clearOn(res, "rp_logout_state");
    return res;
  }
  const res = NextResponse.redirect(new URL("/", req.url), { status: 303 });
  clearOn(res, "rp_access");
  clearOn(res, "rp_id");
  clearOn(res, "rp_refresh");
  clearOn(res, "rp_logout_state");
  return res;
}
