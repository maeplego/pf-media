# pf-media

P03 のメディア基盤です。**本番 CDN / ウイルススキャン基盤の置き換えではありません。**

オブジェクト本体は MinIO（本番は S3 / R2）、メタデータと権限は PostgreSQL、画像派生は非同期 processor（sharp）が担当します。

```
apps/api/        presign / complete / ファイル API（Go）
apps/processor/  画像リサイズ・WebP・EXIF 除去（Node + sharp）
apps/web/        マイドライブ UI（Next.js）
deploy/          Postgres + MinIO + Redis + Compose
```

## 起動

```powershell
copy deploy\.env.example deploy\.env
docker compose -f deploy/compose.yaml --env-file deploy/.env up --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:8090 | media API |
| http://localhost:8091 | MinIO console |
| http://localhost:3004 | マイドライブ UI |

ローカルデモでは UI が `X-Dev-User-Sub` ヘッダでユーザーを切り替えます。IdP 連携は `OIDC_ISSUER` を API に渡すと Bearer JWT を JWKS 検証します（P01）。

Compose では API に `extra_hosts: localhost:host-gateway` を付け、presign URL のホスト（`localhost:9000`）と MinIO 署名が一致するようにしています。

## デモ手順

1. http://localhost:3004 を開く（ユーザー A）
2. 画像をアップロード → 数秒後サムネイル表示
3. 別ユーザー B に切り替え、A のファイルが見えないことを確認

## 契約（他プロジェクト）

- `POST /v1/uploads/presign` — `{ contentType, size, purpose }`
- `POST /v1/uploads/complete` — `{ fileId, etag }`
- `GET /v1/files/:id` — メタデータと派生 URL（署名付き GET）

`purpose`: `wiki`, `product`, `blog`, `chat`, `drive`

## 制限（学習用）

- 最大 20MB / 画像 4000px 超は拒否
- MIME: `image/jpeg`, `image/png`, `image/webp`, `image/gif` のみ（デモ）
- ユーザークォータ既定 100MB
