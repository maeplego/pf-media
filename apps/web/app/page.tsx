import { redirect } from "next/navigation";
import { PollPending } from "./PollPending";

const API = process.env.MEDIA_API_URL || "http://localhost:8090";

type FileView = {
  id: string;
  contentType: string;
  status: string;
  jobId?: string;
  jobStatus?: string;
  jobError?: string;
  variants: Record<string, { url: string; contentType: string }>;
};

async function apiFetch(path: string, sub: string, init?: RequestInit) {
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
    throw new Error(await res.text());
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

type QuotaView = {
  usedBytes: number;
  limitBytes: number;
};

async function listDrive(sub: string): Promise<{ files: FileView[]; quota: QuotaView }> {
  const data = await apiFetch("/v1/files", sub);
  return {
    files: (data.files as FileView[]) || [],
    quota: data.quota || { usedBytes: 0, limitBytes: 0 },
  };
}

function formatBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

async function uploadFile(sub: string, file: File) {
  "use server";
  const presign = await apiFetch("/v1/uploads/presign", sub, {
    method: "POST",
    body: JSON.stringify({
      contentType: file.type,
      size: file.size,
      purpose: "drive",
    }),
  });
  const put = await fetch(presign.uploadUrl, {
    method: "PUT",
    headers: { "Content-Type": file.type },
    body: Buffer.from(await file.arrayBuffer()),
  });
  if (!put.ok) throw new Error("upload failed");
  const etag = put.headers.get("etag") || "";
  await apiFetch("/v1/uploads/complete", sub, {
    method: "POST",
    body: JSON.stringify({ fileId: presign.fileId, etag }),
  });
}

async function createShare(sub: string, fileId: string, expiresInSeconds: number, password: string) {
  "use server";
  const data = await apiFetch("/v1/share-links", sub, {
    method: "POST",
    body: JSON.stringify({ fileId, expiresInSeconds, password }),
  });
  redirect(`/s/${data.token}`);
}

async function retryJob(sub: string, jobId: string) {
  "use server";
  await apiFetch(`/v1/jobs/${jobId}/retry`, sub, { method: "POST", body: "{}" });
}

async function deleteOwnedFile(sub: string, fileId: string) {
  "use server";
  await apiFetch(`/v1/files/${fileId}`, sub, { method: "DELETE" });
}

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ user?: string }>;
}) {
  const sp = await searchParams;
  const user = sp.user || "demo-user-a";
  const { files, quota } = await listDrive(user).catch(() => ({
    files: [] as FileView[],
    quota: { usedBytes: 0, limitBytes: 0 },
  }));
  const pending = files.some((f) => f.status === "pending");

  return (
    <div>
      <p>
        ユーザー: <strong>{user}</strong>{" "}
        <a href="?user=demo-user-a">A</a> · <a href="?user=demo-user-b">B</a>
        {" — "}
        容量 {formatBytes(quota.usedBytes)} / {formatBytes(quota.limitBytes)}
      </p>
      <PollPending active={pending} />
      <form
        action={async (fd) => {
          "use server";
          const f = fd.get("file") as File | null;
          if (!f || f.size === 0) return;
          await uploadFile(user, f);
        }}
      >
        <input type="file" name="file" accept="image/*" required />
        <button type="submit">アップロード</button>
      </form>
      <ul style={{ listStyle: "none", padding: 0, marginTop: "1.5rem" }}>
        {files.map((f) => {
          const thumb = f.variants.thumb?.url || f.variants.orig?.url;
          return (
            <li key={f.id} style={{ marginBottom: "1rem", borderBottom: "1px solid #ddd", paddingBottom: "1rem" }}>
              <div>{f.id.slice(0, 8)}… — {f.status}{f.jobError ? ` (${f.jobError})` : ""}</div>
              {thumb ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={thumb} alt="" style={{ maxWidth: 160, marginTop: 8 }} />
              ) : (
                <em>処理中…</em>
              )}
              {f.status === "failed" && f.jobId ? (
                <form action={async () => { "use server"; await retryJob(user, f.jobId!); }}>
                  <button type="submit">処理を再実行</button>
                </form>
              ) : null}
              <form action={async () => { "use server"; await deleteOwnedFile(user, f.id); }} style={{ marginTop: 8 }}>
                <button type="submit">削除</button>
              </form>
              <div style={{ marginTop: 8 }}>
                <form
                  action={async (fd) => {
                    "use server";
                    const pw = String(fd.get("password") || "");
                    const ttl = Number(fd.get("ttl") || 3600);
                    await createShare(user, f.id, ttl, pw);
                  }}
                  style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}
                >
                  <input type="password" name="password" placeholder="パスワード（任意）" autoComplete="new-password" />
                  <button type="submit" name="ttl" value="3600">1時間共有</button>
                  <button type="submit" name="ttl" value="60">1分で期限切れ</button>
                </form>
              </div>
            </li>
          );
        })}
      </ul>
      <p style={{ color: "#666", fontSize: 14 }}>
        処理中のファイルがあるあいだは数秒ごとに自動更新します。共有 URL はログイン不要です。
      </p>
    </div>
  );
}
