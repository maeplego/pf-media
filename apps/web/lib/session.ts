import { readCookie } from "./oidc/cookies";
import { internalBase, oidcEnabled } from "./oidc/env";

export type OrgMembership = {
  orgId: string;
  orgName: string;
  role: string;
};

export type DriveSession = {
  sub: string;
  accessToken?: string;
  displayName?: string;
  orgId?: string;
  organizations?: OrgMembership[];
  devMode: boolean;
};

export async function getDriveSession(devUser?: string): Promise<DriveSession | null> {
  if (!oidcEnabled()) {
    const saved = (await readCookie("dev_org")) || "";
    const organizations = [
      { orgId: "org-demo-a", orgName: "Demo Org A", role: "owner" },
      { orgId: "org-demo-b", orgName: "Demo Org B", role: "member" },
    ];
    const orgId = saved || organizations[0].orgId;
    return {
      sub: devUser?.trim() || "demo-user-a",
      orgId,
      organizations,
      devMode: true,
    };
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
  const ui = (await res.json()) as {
    sub?: string;
    name?: string;
    email?: string;
    org_id?: string;
    organizations?: { org_id?: string; org_name?: string; role?: string }[];
  };
  if (!ui.sub) {
    return null;
  }
  const organizations = (ui.organizations || [])
    .filter((o) => o.org_id)
    .map((o) => ({
      orgId: String(o.org_id),
      orgName: String(o.org_name || o.org_id),
      role: String(o.role || "member"),
    }));
  return {
    sub: ui.sub,
    accessToken: access,
    displayName: ui.name || ui.email || ui.sub,
    orgId: ui.org_id ? String(ui.org_id) : organizations[0]?.orgId,
    organizations,
    devMode: false,
  };
}

export async function requireDriveSession(devUser?: string): Promise<DriveSession> {
  const session = await getDriveSession(devUser);
  if (!session) {
    throw new Error("unauthorized");
  }
  return session;
}
