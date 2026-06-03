# In CI the environment variables are already exported, so op run is skipped.
# Locally, op run resolves the op:// references in .env before launching.
#
# [Ja] CI では環境変数が既にエクスポート済みのため op run を介しません。
# ローカルでは .env 内の op:// 参照を op run が解決してから起動します。
ifeq ($(CI),true)
  OP_RUN_DEV = APP_ENV=dev
else
  OP_RUN_DEV = APP_ENV=dev op run --env-file=".env" --
endif

.PHONY: help
help: ## Show this help. [Ja] このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Start the HTTP server. [Ja] HTTP サーバーを起動
	$(OP_RUN_DEV) go run ./cmd/server

.PHONY: build
build: ## Build the server binary into bin/server. [Ja] サーバーバイナリを bin/server にビルド
	go build -o bin/server ./cmd/server

.PHONY: templ-generate
templ-generate: ## Generate Go code from templ files (also runs goimports). [Ja] templ ファイルから Go コードを生成 (goimports も実行)
	go tool templ generate
	@$(MAKE) goimports

.PHONY: fmt
fmt: ## Format Go code (go fmt + goimports). [Ja] Go コードをフォーマット (go fmt + goimports)
	go fmt ./...
	@$(MAKE) goimports

.PHONY: goimports
goimports: ## Organize imports with goimports. [Ja] goimports で import 文を整理
	go tool goimports -w .

.PHONY: goimports-check
goimports-check: ## Check imports with goimports (fails on diff). [Ja] goimports で import 文をチェック (差分があれば失敗)
	@unformatted=$$(go tool goimports -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "The following files have incorrect imports:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "Please run 'make goimports' to fix imports"; \
		exit 1; \
	fi

.PHONY: lint
lint: ## Run golangci-lint. [Ja] golangci-lint を実行
	golangci-lint run --config=.golangci.yml ./...

# koryluslint is the Korylus shared linter from korylus-tools. Its `comment`
# subcommand checks misuse of the bilingual [Ja] marker in code comments. The
# version is pinned via the `tool` directive in go.mod and fetched from the
# public module github.com/korylus/tools; `go tool koryluslint` runs that pinned
# binary with go/ as the working directory, so it walks the Go and templ sources.
#
# [Ja] koryluslint は korylus-tools が提供する Korylus 共通リンタ。その `comment`
# サブコマンドはコードコメントの英日併記 [Ja] マーカーの誤用を検査する。バージョンは
# go.mod の tool ディレクティブで固定し、公開モジュール github.com/korylus/tools から
# 取得する。`go tool koryluslint` がその固定版を go/ を作業ディレクトリとして実行する
# ことで、Go・templ ソースを走査する。

.PHONY: lint-comment
lint-comment: ## Check [Ja]-marker misuse in code comments (koryluslint). [Ja] コードコメントの [Ja] マーカー誤用をチェック (koryluslint)
	@go tool koryluslint comment

.PHONY: test
test: ## Run tests. [Ja] テストを実行
	go test -race ./...

# Database commands. db-migrate / db-dump / db-rollback resolve the op:// refs
# in .env via op run; db-new only writes a file and db-setup-test talks to the
# local/CI Postgres directly, so neither needs op.
#
# [Ja] データベースコマンド。db-migrate / db-dump / db-rollback は op run で .env の
# op:// 参照を解決する。db-new はファイルを作るだけ、db-setup-test はローカル/CI の
# Postgres に直接つなぐため、どちらも op を必要としない。

# Strip the random \restrict / \unrestrict lines that pg_dump >= 17.6 emits into
# db/schema.sql, which dbmate writes on both up and dump. Shared by db-migrate
# and db-dump so the workaround lives in one place.
# https://github.com/amacneil/dbmate/issues/678
#
# [Ja] dbmate が up / dump の双方で db/schema.sql へ書き込む、pg_dump 17.6 以降が
# 出力するランダムな \restrict / \unrestrict 行を削除する。db-migrate と db-dump で
# 共有し、回避策を 1 箇所にまとめる。
STRIP_SCHEMA_RESTRICT = sed -i '/^\\restrict /d;/^\\unrestrict /d' db/schema.sql

.PHONY: db-migrate
db-migrate: ## Apply pending migrations and dump the schema. [Ja] 未適用のマイグレーションを実行しスキーマをダンプ
	$(OP_RUN_DEV) dbmate up
	@$(STRIP_SCHEMA_RESTRICT)

.PHONY: db-rollback
db-rollback: ## Roll back the most recent migration. [Ja] 直近のマイグレーションをロールバック
	$(OP_RUN_DEV) dbmate down

.PHONY: db-new
db-new: ## Create a new migration file (usage: make db-new name=create_users). [Ja] 新しいマイグレーションファイルを作成
	dbmate new $(name)

.PHONY: db-dump
db-dump: ## Dump the current database schema to db/schema.sql. [Ja] 現在の DB スキーマを db/schema.sql にダンプ
	$(OP_RUN_DEV) dbmate dump
	@$(STRIP_SCHEMA_RESTRICT)

.PHONY: db-setup-test
db-setup-test: ## Reset the test database and apply db/schema.sql. [Ja] テスト用 DB をリセットしてスキーマを適用
	@if [ "$$CI" = "true" ]; then host=localhost; else host=postgresql; fi; \
	echo "Resetting the test database and applying the schema (host=$$host)"; \
	psql -h $$host -U postgres -d groobb_test -c 'DROP SCHEMA public CASCADE; CREATE SCHEMA public;' > /dev/null 2>&1; \
	psql -h $$host -U postgres -d groobb_test -f db/schema.sql > /dev/null

.PHONY: sqlc-generate
sqlc-generate: ## Generate Go code from SQL queries. [Ja] SQL クエリから Go コードを生成
	sqlc generate
