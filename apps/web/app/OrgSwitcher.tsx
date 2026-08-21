"use client";

import { useTransition } from "react";

type OrgOption = { orgId: string; orgName: string; role: string };

export function OrgSwitcher({
  currentOrgId,
  organizations,
  onSwitch,
}: {
  currentOrgId?: string;
  organizations: OrgOption[];
  onSwitch: (orgId: string) => Promise<void>;
}) {
  const [pending, start] = useTransition();
  if (!organizations.length) return null;
  return (
    <label className="row" style={{ gap: "0.5rem", alignItems: "center" }}>
      <span className="muted">組織</span>
      <select
        value={currentOrgId || organizations[0]?.orgId || ""}
        disabled={pending}
        onChange={(e) => {
          const next = e.target.value;
          start(async () => {
            await onSwitch(next);
          });
        }}
        style={{ width: "auto", minWidth: 160 }}
      >
        {organizations.map((o) => (
          <option key={o.orgId} value={o.orgId}>
            {o.orgName} ({o.role})
          </option>
        ))}
      </select>
    </label>
  );
}
