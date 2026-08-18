# P03 pf-media

製品コード。実装前に `project/media-platform/AGENTS.md` を読む。

テスト: `go test ./...` を `apps/api` で実行（単体・HTTP 統合。Compose 起動中は `e2e` も実 API / Garage を叩く。未起動なら skip）。processor は `apps/processor` で `npm test`。Compose は `deploy/`。
