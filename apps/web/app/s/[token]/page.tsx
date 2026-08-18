const API = process.env.MEDIA_API_URL || "http://localhost:8090";

type PublicFile = {
  contentType: string;
  status: string;
  expiresAt: string;
  variants: Record<string, { url: string; contentType: string }>;
};

export default async function SharePage({
  params,
}: {
  params: Promise<{ token: string }>;
}) {
  const { token } = await params;
  const res = await fetch(`${API}/v1/s/${encodeURIComponent(token)}`, { cache: "no-store" });
  if (res.status === 410) {
    return <p>この共有リンクは期限切れです。</p>;
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
      <p>
        <a href={`${API}/v1/s/${encodeURIComponent(token)}/download`}>原画をダウンロード</a>
      </p>
      <p style={{ color: "#666", fontSize: 14 }}>期限: {data.expiresAt}</p>
    </div>
  );
}
