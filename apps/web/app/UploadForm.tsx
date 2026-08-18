"use client";

import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { uploadDriveFile } from "./actions";

export function UploadForm({ folderId, devUser }: { folderId: string; devUser?: string }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    try {
      const message = await uploadDriveFile(new FormData(e.currentTarget), devUser);
      if (message) {
        setError(message);
        return;
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "upload failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form onSubmit={onSubmit}>
      {folderId ? <input type="hidden" name="folderId" value={folderId} /> : null}
      <input type="file" name="file" accept="image/*" required disabled={busy} />
      <button type="submit" disabled={busy}>
        {busy ? "送信中…" : "アップロード"}
      </button>
      {error ? <p style={{ color: "crimson" }}>{error}</p> : null}
    </form>
  );
}
