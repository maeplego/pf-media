# pf-media

P03 のメディア基盤です。**本番 CDN / ウイルススキャン基盤の置き換えではありません。**

人間向け書類: `project/portfolio-plan/media-platform/docs/`。オブジェクトは開発時 **Garage**（S3 互換）。メタデータは PostgreSQL、画像派生は非同期 processor。

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

ローカルデモ（既定）では UI が `?user=` と `X-Dev-User-Sub` でユーザーを切り替えます。P01 IdP 連携時は web に `OIDC_ISSUER` と `OIDC_CLIENT_ID` を渡し、API に `OIDC_ISSUER`（Compose 内は `OIDC_INTERNAL_BASE`）と `MEDIA_DEV_AUTH=false` を設定します。API は JWT（ID token）または opaque access token（`/userinfo`）で `sub` を解決します。

### OIDC デモ（pf-identity と併用）

1. `pf-identity/deploy` で IdP を起動（http://localhost:8080）
2. 管理 UI（http://localhost:3002）で public クライアント `pf-media-web` を登録。redirect: `http://localhost:3004/callback`
3. `deploy/.env` に OIDC 変数を設定（`.env.example` 参照）し `MEDIA_DEV_AUTH=false`
4. media Compose を再ビルド。http://localhost:3004 → IdP ログイン → マイドライブ

Compose では API と web に `extra_hosts: localhost:host-gateway` を付けています。presign URL のホスト（`localhost:3900`）と Garage 署名を揃え、ドライブ UI からの PUT がコンテナ内の `localhost` ではなくホスト上の Garage に届くようにするためです。S3 クライアントは path-style アクセスを使います（Garage 要件）。

## テスト

```powershell
go test ./...   # apps/api。単体 + HTTP 統合。Compose 起動中は e2e も実行
npm test        # apps/processor
```

## デモ手順

1. http://localhost:3004 を開く（ユーザー A）
2. 画像をアップロード → 処理中は自動更新されサムネイルが表示される。容量は画面上部
3. 「1時間共有」で公開ページへ。パスワード欄は空でも、入れて守ってもよい
4. 「1分で期限切れ」のリンクは、期限後に 410 になる
5. 別ユーザー B に切り替え、A のマイドライブ一覧には A のファイルが出ない
6. 「削除」でオブジェクトと容量を返す
7. 「フォルダ作成」で階層を作り、中にアップロードする

## 契約（他プロジェクト）

- `POST /v1/uploads/presign` — `{ contentType, size, purpose }`
- `POST /v1/uploads/complete` — `{ fileId, etag }`
- `GET /v1/files` — 所有者の一覧と `quota`（`folderId` でフォルダ内。ルートは空）
- `GET /v1/files/:id` — メタデータと派生 URL（署名付き GET）
- `DELETE /v1/files/:id` — 所有者のみ。派生オブジェクトを消しクォータを返す
- `GET /v1/quota` — `{ usedBytes, limitBytes }`
- `POST /v1/folders` — `{ name, parentId? }`
- `GET /v1/folders` — `parentId` 配下（ルートは空）
- `DELETE /v1/folders/:id` — 所有者のみ。中のファイル・サブフォルダを再帰削除しクォータを返す。冪等
- `POST /v1/share-links` — `{ fileId, expiresInSeconds, password? }`（所有者のみ。password は任意）
- `GET /v1/s/:token` — ログイン不要。期限切れは 410。パスワード付きは `X-Share-Password` が必要（無いと 401）
- `GET /v1/s/:token/download` — 署名付き GET へ 302（同じパスワードヘッダ）

- `GET /v1/jobs/:id` — 所有者のみ。状態とエラー
- `POST /v1/jobs/:id/retry` — 失敗ジョブを本線キューへ戻す

`purpose`: `wiki`, `product`, `blog`, `chat`, `drive`

## 制限（学習用）

- 最大 20MB は presign / complete で HTTP 413
- 画像 4000px 超・非画像マジックバイトは processor が拒否し、失敗ジョブは Redis の `media:jobs:dlq` に残る
- MIME: 画像（jpeg/png/webp/gif）と `application/pdf` / `application/zip` / `text/plain`。complete 時に先頭バイトで検証。非画像は processor をスキップし `ready` + 原画 URL
- ユーザークォータ既定 100MB
