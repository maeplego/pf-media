import { readCookie } from "./oidc/cookies";
import { internalBase, oidcEnabled } from "./oidc/env";

export type DriveSession = {
  sub: string;
  accessToken?: string;
  displayName?: string;
  devMode: boolean;
};

export async function getDriveSession(devUser?: string): Promise<DriveSession | null> {
  if (!oidcEnabled()) {
    return { sub: devUser?.trim() || "demo-user-a", devMode: true };
  }
  const access = await readCookie("rp_access");
  if (!access) {
    return null;
  }
  const res = await fetch(`${internalBase()}/userinfo`, {
    headers: { Authorization: `Bearer ${access}` },
    cache: "no-store",
  });
  if (!res.ok) {
    return null;
  }
  const ui = (await res.json()) as { sub?: string; name?: string; email?: string };
  if (!ui.sub) {
    return null;
  }
  const displayName = ui.name || ui.email || ui.sub;
  return { sub: ui.sub, accessToken: access, displayName, devMode: false };
}

export async function requireDriveSession(devUser?: string): Promise<DriveSession> {
  const session = await getDriveSession(devUser);
  if (!session) {
    throw new Error("unauthorized");
  }
  return session;
}
