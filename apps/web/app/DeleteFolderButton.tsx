"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";
import { deleteDriveFolder } from "./actions";

export function DeleteFolderButton({
  user,
  folderId,
  folderName,
}: {
  user: string;
  folderId: string;
  folderName: string;
}) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const label = folderName.trim() || "このフォルダ";

  async function doDelete() {
    setConfirmOpen(false);
    setError(null);
    setBusy(true);
    try {
      const message = await deleteDriveFolder(user, folderId);
      if (message) {
        setError(message);
        return;
      }
      router.refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "folder delete failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <span style={{ marginLeft: 8 }}>
      <button type="button" onClick={() => setConfirmOpen(true)} disabled={busy}>
        {busy ? "削除中…" : "削除"}
      </button>
      {error ? <span style={{ color: "crimson", marginLeft: 8 }}>{error}</span> : null}
      {confirmOpen ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-labelledby={`delete-folder-${folderId}`}
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.45)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1000,
          }}
          onClick={() => setConfirmOpen(false)}
        >
          <div
            style={{
              background: "#fff",
              padding: "1.25rem 1.5rem",
              borderRadius: 8,
              maxWidth: 420,
              boxShadow: "0 8px 32px rgba(0,0,0,0.2)",
            }}
            onClick={(e) => e.stopPropagation()}
          >
            <p id={`delete-folder-${folderId}`} style={{ margin: "0 0 1rem" }}>
              「{label}」を削除しますか？
              <br />
              <br />
              中のファイルとサブフォルダもすべて削除されます。この操作は取り消せません。
            </p>
            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
              <button type="button" onClick={() => setConfirmOpen(false)} disabled={busy}>
                キャンセル
              </button>
              <button type="button" onClick={doDelete} disabled={busy}>
                削除する
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </span>
  );
}
