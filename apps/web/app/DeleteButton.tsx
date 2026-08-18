"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { deleteDriveFile } from "./actions";

export function DeleteButton({ user, fileId }: { user: string; fileId: string }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onClick() {
    setError(null);
    setBusy(true);
    try {
      const message = await deleteDriveFile(user, fileId);
      if (message) {
        setError(message);
        return;
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div style={{ marginTop: 8 }}>
      <button type="button" onClick={onClick} disabled={busy}>
        {busy ? "削除中…" : "削除"}
      </button>
      {error ? <p style={{ color: "crimson" }}>{error}</p> : null}
    </div>
  );
}
