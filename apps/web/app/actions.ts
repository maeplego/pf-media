"use server";

import { revalidatePath } from "next/cache";

const API = process.env.MEDIA_API_URL || "http://localhost:8090";

function revalidateDrive() {
  revalidatePath("/", "page");
}

export async function apiFetch(path: string, sub: string, init?: RequestInit) {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      ...(init?.headers || {}),
      "X-Dev-User-Sub": sub,
      "Content-Type": "application/json",
    },
    cache: "no-store",
  });
  if (!res.ok) {
    throw new Error((await res.text()) || res.statusText);
  }
  if (res.status === 204) {
    return {};
  }
  const text = await res.text();
  if (!text) {
    return {};
  }
  return JSON.parse(text);
}

/** PUT は署名 URL（localhost:3900）へ。web コンテナは extra_hosts でホストの Garage に届ける。 */
export async function uploadDriveFile(sub: string, formData: FormData): Promise<string | null> {
  const file = formData.get("file");
  if (!(file instanceof File) || file.size === 0) {
    return "ファイルを選んでください";
  }
  const folderId = String(formData.get("folderId") || "");
  try {
    const presign = await apiFetch("/v1/uploads/presign", sub, {
      method: "POST",
      body: JSON.stringify({
        contentType: file.type || "application/octet-stream",
        size: file.size,
        purpose: "drive",
        folderId,
      }),
    });
    const put = await fetch(presign.uploadUrl, {
      method: "PUT",
      headers: { "Content-Type": file.type || "application/octet-stream" },
      body: Buffer.from(await file.arrayBuffer()),
    });
    if (!put.ok) {
      return `object upload failed (${put.status})`;
    }
    await apiFetch("/v1/uploads/complete", sub, {
      method: "POST",
      body: JSON.stringify({ fileId: presign.fileId, etag: put.headers.get("etag") || "" }),
    });
    revalidateDrive();
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : "upload failed";
  }
}

export async function createDriveFolder(sub: string, name: string, parentId: string): Promise<string | null> {
  const trimmed = name.trim();
  if (!trimmed) {
    return "フォルダ名を入力してください";
  }
  try {
    await apiFetch("/v1/folders", sub, {
      method: "POST",
      body: JSON.stringify({ name: trimmed, parentId }),
    });
    revalidateDrive();
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : "folder create failed";
  }
}

export async function deleteDriveFile(sub: string, fileId: string): Promise<string | null> {
  try {
    await apiFetch(`/v1/files/${fileId}`, sub, { method: "DELETE" });
    revalidateDrive();
    return null;
  } catch (err) {
    const message = err instanceof Error ? err.message : "delete failed";
    if (message.toLowerCase().includes("not found")) {
      return null;
    }
    return message;
  }
}

export async function deleteDriveFolder(sub: string, folderId: string): Promise<string | null> {
  try {
    await apiFetch(`/v1/folders/${folderId}`, sub, { method: "DELETE" });
    revalidateDrive();
    return null;
  } catch (err) {
    const message = err instanceof Error ? err.message : "folder delete failed";
    if (message.toLowerCase().includes("not found")) {
      return null;
    }
    return message;
  }
}
