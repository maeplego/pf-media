import { redirect } from "next/navigation";
import { apiFetch } from "./actions";
import { DeleteButton } from "./DeleteButton";
import { PollPending } from "./PollPending";
import { UploadForm } from "./UploadForm";

type FileView = {
  id: string;
  contentType: string;
  status: string;
  jobId?: string;
  jobStatus?: string;
  jobError?: string;
  sizeBytes?: number;
  variants: Record<string, { url: string; contentType: string }>;
};

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
        （A のファイルは B には出ません）
      </p>
      <p>
        容量 <strong>{formatBytes(quota.usedBytes)}</strong> / {formatBytes(quota.limitBytes)}
        <span style={{ color: "#666" }}>（アップロードで増え、削除で戻ります）</span>
      </p>
      <PollPending active={pending} />
      <UploadForm user={user} />
      <ul style={{ listStyle: "none", padding: 0, marginTop: "1.5rem" }}>
        {files.map((f) => {
          const thumb = f.variants.thumb?.url || f.variants.orig?.url;
          return (
            <li key={f.id} style={{ marginBottom: "1rem", borderBottom: "1px solid #ddd", paddingBottom: "1rem" }}>
              <div>
                {f.id.slice(0, 8)}… — {f.status} ({formatBytes(f.sizeBytes || 0)})
                {f.jobError ? ` (${f.jobError})` : ""}
              </div>
              {thumb ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={thumb} alt="" width={160} style={{ width: 160, height: "auto", marginTop: 8, background: "#eee" }} />
              ) : (
                <em>処理中… サムネは processor 完了後に出ます</em>
              )}
              {f.status === "failed" && f.jobId ? (
                <form action={async () => { "use server"; await retryJob(user, f.jobId!); }}>
                  <button type="submit">処理を再実行</button>
                </form>
              ) : null}
              <DeleteButton user={user} fileId={f.id} />
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
        処理中のファイルがあるあいだは数秒ごとに自動更新します。数バイトのテスト画像だとサムネはほぼ見えません。共有 URL はログイン不要です。
      </p>
    </div>
  );
}
