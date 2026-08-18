import { redirect } from "next/navigation";

const API = process.env.MEDIA_API_URL || "http://localhost:8090";

type FileView = {
  id: string;
  contentType: string;
  status: string;
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
  return res.json();
}

async function listFiles(sub: string): Promise<FileView[]> {
  const data = await apiFetch("/v1/files", sub);
  return data.files as FileView[];
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

async function createShare(sub: string, fileId: string, expiresInSeconds: number) {
  "use server";
  const data = await apiFetch("/v1/share-links", sub, {
    method: "POST",
    body: JSON.stringify({ fileId, expiresInSeconds }),
  });
  redirect(`/s/${data.token}`);
}

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ user?: string }>;
}) {
  const sp = await searchParams;
  const user = sp.user || "demo-user-a";
  const files = await listFiles(user).catch(() => [] as FileView[]);

  return (
    <div>
      <p>
        ユーザー: <strong>{user}</strong>{" "}
        <a href="?user=demo-user-a">A</a> · <a href="?user=demo-user-b">B</a>
      </p>
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
              <div>{f.id.slice(0, 8)}… — {f.status}</div>
              {thumb ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={thumb} alt="" style={{ maxWidth: 160, marginTop: 8 }} />
              ) : (
                <em>処理中…</em>
              )}
              <div style={{ marginTop: 8, display: "flex", gap: 8 }}>
                <form action={async () => { "use server"; await createShare(user, f.id, 3600); }}>
                  <button type="submit">1時間共有</button>
                </form>
                <form action={async () => { "use server"; await createShare(user, f.id, 60); }}>
                  <button type="submit">1分で期限切れ</button>
                </form>
              </div>
            </li>
          );
        })}
      </ul>
      <p style={{ color: "#666", fontSize: 14 }}>
        数秒後にリロードするとサムネイルが表示されます。共有 URL はログイン不要です。ユーザー B で開いても同じトークンなら見えます。
      </p>
    </div>
  );
}
