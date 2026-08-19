# pf-media

学習用のメディア基盤です。アップロード用の署名 URL、メタデータ（PostgreSQL）、画像のリサイズ / WebP / EXIF 除去を非同期で行います。オブジェクト保存は開発時 Garage（S3 互換）です。**本番 CDN やウイルススキャン基盤の置き換えではありません。**

| ディレクトリ | 役割 |
| --- | --- |
| `apps/api` | 署名・完了・ファイル API（Go） |
| `apps/processor` | 画像処理（Node + sharp） |
| `apps/web` | マイドライブ UI（Next.js） |
| `deploy/` | Postgres、Garage、Redis、Compose |

## 起動

```powershell
copy deploy\.env.example deploy\.env
docker compose -f deploy/compose.yaml --env-file deploy/.env up --build
```

| URL | 用途 |
| --- | --- |
| http://localhost:3004 | マイドライブ |
| http://localhost:8090 | media API |
| http://localhost:3900 | Garage S3 API |

既定では `?user=` と開発ヘッダでユーザーを切り替えます。本番アカウントではありません。

## デモ

1. http://localhost:3004 を開く
2. 画像をアップロードする。処理が終わるとサムネイルが出ます
3. 「1時間共有」で公開ページを開く（パスワードは任意）
4. 「1分で期限切れ」のリンクは、期限後に 410 になります
5. 別ユーザーに切り替えると、先のファイルは一覧に出ません

容量の既定上限はユーザーあたり 100MB、1 ファイル 20MB です。受け付ける種別は画像（jpeg/png/webp/gif）と PDF / zip / テキストです。

## テスト

```powershell
cd apps/api
go test ./...
cd ..\processor
npm test
```

Compose 起動中は API の e2e も実 Garage を叩きます。未起動なら skip します。

## OpenID Connect（任意）

[pf-identity](https://github.com/maeplego/pf-identity) を起動し、管理画面で public クライアント `pf-media-web`（redirect `http://localhost:3004/callback`）を登録します。`deploy/.env` に OIDC 変数を入れ、`MEDIA_DEV_AUTH=false` にして Compose を再ビルドします。

設計の詳細は [portfolio-plan](https://github.com/maeplego/portfolio-plan) の `portfolio-plan/media-platform/docs/` です。
