"use client";

import { useRouter } from "next/navigation";
import { FormEvent, useState } from "react";
import { createDriveFolder } from "./actions";

export function CreateFolderForm({ user, parentId }: { user: string; parentId: string }) {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const name = String(new FormData(e.currentTarget).get("name") || "");
    const message = await createDriveFolder(user, name, parentId);
    if (message) {
      setError(message);
      return;
    }
    e.currentTarget.reset();
    setError(null);
    router.refresh();
  }

  return (
    <form onSubmit={onSubmit} style={{ display: "flex", gap: 8, alignItems: "center", marginTop: 8 }}>
      <input name="name" placeholder="フォルダ名" required />
      <button type="submit">フォルダ作成</button>
      {error ? <span style={{ color: "crimson" }}>{error}</span> : null}
    </form>
  );
}
