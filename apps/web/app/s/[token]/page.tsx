import { cookies } from "next/headers";
import { redirect } from "next/navigation";

const API = process.env.MEDIA_API_URL || "http://localhost:8090";

type PublicFile = {
  contentType: string;
  status: string;
  expiresAt: string;
  variants: Record<string, { url: string; contentType: string }>;
};

async function submitPassword(token: string, formData: FormData) {
  "use server";
  const password = String(formData.get("password") || "");
  const jar = await cookies();
  jar.set("sharepw", password, {
    httpOnly: true,
    path: `/s/${token}`,
    maxAge: 600,
    sameSite: "lax",
  });
  redirect(`/s/${token}`);
}

export default async function SharePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const jar = await cookies();
  const password = jar.get("sharepw")?.value || "";
  const res = await fetch(`${API}/v1/s/${encodeURIComponent(token)}`, {
    cache: "no-store",
    headers: password ? { "X-Share-Password": password } : {},
  });
  if (res.status === 410) {
    return <p>この共有リンクは期限切れです。</p>;
  }
  if (res.status === 401) {
    return (
      <form action={submitPassword.bind(null, token)}>
        <p>{password ? "パスワードが違います。" : "このリンクはパスワード付きです。"}</p>
        <input type="password" name="password" required autoComplete="current-password" />
        <button type="submit">開く</button>
      </form>
    );
  }
  if (!res.ok) {
    return <p>リンクが見つかりません。</p>;
  }
  const data = (await res.json()) as PublicFile;
  const img = data.variants.thumb?.url || data.variants.orig?.url;
  return (
    <div>
      <p>ログイン不要の公開ページ</p>
      {img ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={img} alt="" style={{ maxWidth: 480 }} />
      ) : (
        <em>処理中…</em>
      )}
      {data.variants.orig?.url ? (
        <p>
          <a href={data.variants.orig.url}>原画をダウンロード</a>
        </p>
      ) : null}
      <p style={{ color: "#666", fontSize: 14 }}>期限: {data.expiresAt}</p>
    </div>
  );
}
