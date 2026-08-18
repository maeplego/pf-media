"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { type DriveSession, requireDriveSession } from "../lib/session";

const API = process.env.MEDIA_API_URL || "http://localhost:8090";

function revalidateDrive() {
  revalidatePath("/", "page");
}

function authHeaders(session: DriveSession): Record<string, string> {
  if (session.accessToken) {
    return { Authorization: `Bearer ${session.accessToken}` };
  }
  return { "X-Dev-User-Sub": session.sub };
}

async function apiFetch(path: string, session: DriveSession, init?: RequestInit) {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      ...(init?.headers || {}),
      ...authHeaders(session),
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

export async function apiFetchForPage(path: string, devUser?: string) {
  const session = await requireDriveSession(devUser);
  return apiFetch(path, session);
}

/** PUT は署名 URL（localhost:3900）へ。web コンテナは extra_hosts でホストの Garage に届ける。 */
export async function uploadDriveFile(formData: FormData, devUser?: string): Promise<string | null> {
  const session = await requireDriveSession(devUser);
  const file = formData.get("file");
  if (!(file instanceof File) || file.size === 0) {
    return "ファイルを選んでください";
  }
  const folderId = String(formData.get("folderId") || "");
  try {
    const presign = await apiFetch("/v1/uploads/presign", session, {
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
    await apiFetch("/v1/uploads/complete", session, {
      method: "POST",
      body: JSON.stringify({ fileId: presign.fileId, etag: put.headers.get("etag") || "" }),
    });
    revalidateDrive();
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : "upload failed";
  }
}

export async function createDriveFolder(name: string, parentId: string, devUser?: string): Promise<string | null> {
  const session = await requireDriveSession(devUser);
  const trimmed = name.trim();
  if (!trimmed) {
    return "フォルダ名を入力してください";
  }
  try {
    await apiFetch("/v1/folders", session, {
      method: "POST",
      body: JSON.stringify({ name: trimmed, parentId }),
    });
    revalidateDrive();
    return null;
  } catch (err) {
    return err instanceof Error ? err.message : "folder create failed";
  }
}

export async function deleteDriveFile(fileId: string, devUser?: string): Promise<string | null> {
  const session = await requireDriveSession(devUser);
  try {
    await apiFetch(`/v1/files/${fileId}`, session, { method: "DELETE" });
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

export async function deleteDriveFolder(folderId: string, devUser?: string): Promise<string | null> {
  const session = await requireDriveSession(devUser);
  try {
    await apiFetch(`/v1/folders/${folderId}`, session, { method: "DELETE" });
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

export async function retryDriveJob(jobId: string, devUser?: string) {
  const session = await requireDriveSession(devUser);
  await apiFetch(`/v1/jobs/${jobId}/retry`, session, { method: "POST", body: "{}" });
  revalidateDrive();
}

export async function createDriveShare(fileId: string, expiresInSeconds: number, password: string, devUser?: string) {
  const session = await requireDriveSession(devUser);
  const data = await apiFetch("/v1/share-links", session, {
    method: "POST",
    body: JSON.stringify({ fileId, expiresInSeconds, password }),
  });
  redirect(`/s/${data.token}`);
}

export async function submitDriveShare(fileId: string, devUser: string | undefined, formData: FormData) {
  const pw = String(formData.get("password") || "");
  const ttl = Number(formData.get("ttl") || 3600);
  await createDriveShare(fileId, ttl, pw, devUser);
}
