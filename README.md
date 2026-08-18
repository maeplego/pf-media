# pf-media

P03 のメディア基盤です。**本番 CDN / ウイルススキャン基盤の置き換えではありません。**

オブジェクト本体は開発時 **Garage**（S3 互換）、本番は S3 / R2。メタデータと権限は PostgreSQL、画像派生は非同期 processor（sharp）が担当します。

```
apps/api/        presign / complete / ファイル API（Go）
apps/processor/  画像リサイズ・WebP・EXIF 除去（Node + sharp）
apps/web/        マイドライブ UI（Next.js）
deploy/          Postgres + Garage + Redis + Compose
```

## 起動

```powershell
copy deploy\.env.example deploy\.env
docker compose -f deploy/compose.yaml --env-file deploy/.env up --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:8090 | media API |
| http://localhost:3900 | Garage S3 API |
| http://localhost:3004 | マイドライブ UI |

ローカルデモでは UI が `X-Dev-User-Sub` ヘッダでユーザーを切り替えます。IdP 連携は `OIDC_ISSUER` を API に渡すと Bearer JWT を JWKS 検証します（P01）。

Compose では API に `extra_hosts: localhost:host-gateway` を付け、presign URL のホスト（`localhost:3900`）と Garage 署名が一致するようにしています。S3 クライアントは path-style アクセスを使います（Garage 要件）。

## デモ手順

1. http://localhost:3004 を開く（ユーザー A）
2. 画像をアップロード → 数秒後サムネイル表示
3. 「1時間共有」で公開ページへ。パスワード欄は空でも、入れて守ってもよい
4. 「1分で期限切れ」のリンクは、期限後に 410 になる
5. 別ユーザー B に切り替え、A のマイドライブ一覧には A のファイルが出ない

## 契約（他プロジェクト）

- `POST /v1/uploads/presign` — `{ contentType, size, purpose }`
- `POST /v1/uploads/complete` — `{ fileId, etag }`
- `GET /v1/files/:id` — メタデータと派生 URL（署名付き GET）
- `POST /v1/share-links` — `{ fileId, expiresInSeconds, password? }`（所有者のみ。password は任意）
- `GET /v1/s/:token` — ログイン不要。期限切れは 410。パスワード付きは `X-Share-Password` が必要（無いと 401）
- `GET /v1/s/:token/download` — 署名付き GET へ 302（同じパスワードヘッダ）

- `GET /v1/jobs/:id` — 所有者のみ。状態とエラー
- `POST /v1/jobs/:id/retry` — 失敗ジョブを本線キューへ戻す

`purpose`: `wiki`, `product`, `blog`, `chat`, `drive`

## 制限（学習用）

- 最大 20MB は presign / complete で HTTP 413
- 画像 4000px 超・非画像マジックバイトは processor が拒否し、失敗ジョブは Redis の `media:jobs:dlq` に残る
- MIME: `image/jpeg`, `image/png`, `image/webp`, `image/gif` のみ（デモ）
- ユーザークォータ既定 100MB
