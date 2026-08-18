import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage, retryDriveJob, submitDriveShare } from "./actions";
import { CreateFolderForm } from "./CreateFolderForm";
import { DeleteButton } from "./DeleteButton";
import { DeleteFolderButton } from "./DeleteFolderButton";
import { PollPending } from "./PollPending";
import { UploadForm } from "./UploadForm";
import { getDriveSession } from "../lib/session";
import { oidcEnabled } from "../lib/oidc/env";

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

type FolderView = {
  id: string;
  parentId?: string;
  name: string;
};

async function listDrive(devUser: string | undefined, folderId: string): Promise<{ files: FileView[]; folders: FolderView[]; quota: QuotaView }> {
  const q = folderId ? `?folderId=${encodeURIComponent(folderId)}` : "";
  const data = await apiFetchForPage(`/v1/files${q}`, devUser);
  return {
    files: (data.files as FileView[]) || [],
    folders: (data.folders as FolderView[]) || [],
    quota: data.quota || { usedBytes: 0, limitBytes: 0 },
  };
}

function driveHref(folder?: string, devUser?: string) {
  const q = new URLSearchParams();
  if (devUser) {
    q.set("user", devUser);
  }
  if (folder) {
    q.set("folder", folder);
  }
  const s = q.toString();
  return s ? `?${s}` : "/";
}

function formatBytes(n: number) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ user?: string; folder?: string; error?: string }>;
}) {
  noStore();
  const sp = await searchParams;
  const folderId = sp.folder || "";
  const session = await getDriveSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const { files, folders, quota } = await listDrive(devUser, folderId).catch(() => ({
    files: [] as FileView[],
    folders: [] as FolderView[],
    quota: { usedBytes: 0, limitBytes: 0 },
  }));
  const current = folderId
    ? await apiFetchForPage(`/v1/folders/${folderId}`, devUser).catch(() => null)
    : null;
  const pending = files.some((f) => f.status === "pending");

  return (
    <div>
      <p>
        ユーザー: <strong>{session!.displayName || session!.sub}</strong>
        {session!.devMode ? (
          <>
            {" "}
            <a href={driveHref(undefined, "demo-user-a")}>A</a> · <a href={driveHref(undefined, "demo-user-b")}>B</a>
            <span style={{ color: "#666" }}>（開発モード: A のファイルは B には出ません）</span>
          </>
        ) : (
          <>
            {" "}
            <form action="/logout" method="post" style={{ display: "inline" }}>
              <button type="submit">ログアウト</button>
            </form>
          </>
        )}
      </p>
      {sp.error ? (
        <p role="alert" style={{ color: "crimson" }}>
          ログインエラー: {sp.error}
        </p>
      ) : null}
      <p>
        容量 <strong>{formatBytes(quota.usedBytes)}</strong> / {formatBytes(quota.limitBytes)}
        <span style={{ color: "#666" }}>（アップロードで増え、削除で戻ります）</span>
      </p>
      <p>
        <a href={driveHref(undefined, devUser)}>ルート</a>
        {current?.parentId ? (
          <>
            {" / "}
            <a href={driveHref(current.parentId, devUser)}>上へ</a>
          </>
        ) : null}
        {current?.name ? ` / ${current.name}` : ""}
      </p>
      <PollPending active={pending} />
      <UploadForm folderId={folderId} devUser={devUser} />
      <CreateFolderForm parentId={folderId} devUser={devUser} />
      <ul style={{ listStyle: "none", padding: 0, marginTop: "1.5rem" }}>
        {folders.map((dir) => (
          <li key={dir.id} style={{ marginBottom: "0.75rem" }}>
            📁 <a href={driveHref(dir.id, devUser)}>{dir.name}</a>
            <DeleteFolderButton folderId={dir.id} folderName={dir.name} devUser={devUser} />
          </li>
        ))}
        {files.map((f) => {
          const thumb = f.variants.thumb?.url;
          const download = f.variants.orig?.url;
          const isImage = f.contentType.startsWith("image/");
          return (
            <li key={f.id} style={{ marginBottom: "1rem", borderBottom: "1px solid #ddd", paddingBottom: "1rem" }}>
              <div>
                {f.id.slice(0, 8)}… — {f.status} ({formatBytes(f.sizeBytes || 0)}) — {f.contentType}
                {f.jobError ? ` (${f.jobError})` : ""}
              </div>
              {thumb ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img src={thumb} alt="" width={160} style={{ width: 160, height: "auto", marginTop: 8, background: "#eee" }} />
              ) : isImage ? (
                <em>処理中… サムネは processor 完了後に出ます</em>
              ) : download ? (
                <p style={{ marginTop: 8 }}>
                  <a href={download} target="_blank" rel="noreferrer">
                    ダウンロード
                  </a>
                </p>
              ) : null}
              {f.status === "failed" && f.jobId ? (
                <form action={retryDriveJob.bind(null, f.jobId, devUser)}>
                  <button type="submit">処理を再実行</button>
                </form>
              ) : null}
              <DeleteButton fileId={f.id} devUser={devUser} />
              <div style={{ marginTop: 8 }}>
                <form action={submitDriveShare.bind(null, f.id, devUser)} style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
                  <input type="password" name="password" placeholder="パスワード（任意）" autoComplete="new-password" />
                  <button type="submit" name="ttl" value="3600">
                    1時間共有
                  </button>
                  <button type="submit" name="ttl" value="60">
                    1分で期限切れ
                  </button>
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
