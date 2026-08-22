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

.PHONY: browse-login
browse-login: ## Log in to the dev site in a browser (other user: make browse-login BROWSE_USER=2). [Ja] dev サイトへブラウザでログイン (別ユーザーは make browse-login BROWSE_USER=2)
	$(OP_RUN_DEV) bash scripts/browse.sh login $(BROWSE_USER)

.PHONY: browse-shot
browse-shot: ## Open a page with the logged-in session and screenshot it (e.g. make browse-shot URL=/). [Ja] ログイン済みセッションでページを開きスクショ (例: make browse-shot URL=/)
	bash scripts/browse.sh shot "$(URL)"

.PHONY: browse-close
browse-close: ## Close the browser session and remove its credentials, origin file and profile (screenshots are kept). [Ja] ブラウザセッションを閉じ、資格情報・origin ファイル・profile を削除 (スクショは残す)
	bash scripts/browse.sh close

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

# A migrated SQLite schema snapshot is built once per test binary and copied
# into a separate database file for each test, so the suite needs neither a
# database prepared beforehand nor any environment variable, and runs without
# op. tparse formats the -json stream; it ships via the go.mod tool directive,
# matching how the other dev tools are invoked here. set -o pipefail surfaces a
# go test failure through the tparse pipe; otherwise make sees only the tparse
# exit code and reads a failing run as success.
#
# [Ja] マイグレーション済みの SQLite スキーマスナップショットをテストバイナリごとに
# 1 度だけ作り、各テスト専用のデータベースファイルへ複製するため、テストスイートは
# 事前に用意されたデータベースも環境変数も必要とせず、op を介さずに走る。
# tparse は -json ストリームを整形するツールで、go.mod の tool ディレクティブ経由で
# 導入し、ここでの他の開発ツールと同じ呼び出し方に揃えている。set -o pipefail は
# go test の失敗を tparse パイプ越しに伝播させる。これが無いと make は tparse の
# 終了コードしか見ず、失敗した実行を成功と誤認してしまう。
.PHONY: test
test: ## Run tests. [Ja] テストを実行
	@bash -c 'set -o pipefail && go test -json -race ./... | go tool tparse -all'

.PHONY: test-pkg
test-pkg: ## Run tests for a single package (usage: make test-pkg PKG=internal/handler/sign_in). [Ja] 特定パッケージのテストを実行
	@if [ -z "$(PKG)" ]; then \
		echo "Error: set the PKG variable"; \
		echo "Usage: make test-pkg PKG=internal/handler/sign_in"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && go test -json -race ./$(PKG) | go tool tparse -all'

.PHONY: test-run
test-run: ## Run a single test (usage: make test-run PKG=internal/handler/sign_in RUN=TestCreate). [Ja] 特定のテストを実行
	@if [ -z "$(PKG)" ]; then \
		echo "Error: set the PKG variable"; \
		echo "Usage: make test-run PKG=internal/handler/sign_in RUN=TestCreate"; \
		exit 1; \
	fi
	@if [ -z "$(RUN)" ]; then \
		echo "Error: set the RUN variable"; \
		echo "Usage: make test-run PKG=internal/handler/sign_in RUN=TestCreate"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && go test -json -race ./$(PKG) -run $(RUN) | go tool tparse -all'

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose logs (for debugging). [Ja] 詳細ログ付きで全テストを実行 (デバッグ用)
	@bash -c 'set -o pipefail && go test -json -race -count=1 ./... | go tool tparse -all'

# Database commands. db-migrate / db-rollback read the database path from .env,
# which op run loads before the command starts; db-new only writes a file, so it
# needs neither.
#
# [Ja] データベースコマンド。db-migrate / db-rollback はデータベースのパスを .env
# から読み、op run がコマンドの起動前にそれを読み込む。db-new はファイルを作るだけの
# ため、どちらも必要としない。

.PHONY: db-migrate
db-migrate: ## Apply pending migrations. [Ja] 未適用のマイグレーションを実行
	$(OP_RUN_DEV) go run ./cmd/migrate up

.PHONY: db-rollback
db-rollback: ## Roll back the most recent migration. [Ja] 直近のマイグレーションをロールバック
	$(OP_RUN_DEV) go run ./cmd/migrate down

# The generated file holds the two annotations goose reads to split a migration
# into its up and down halves, under a UTC timestamp that fixes the order the
# migrations are applied in.
#
# [Ja] 生成されるファイルは、goose がマイグレーションを up と down の 2 つに分けて読む
# ための注釈 2 行を持つ。ファイル名の先頭に付ける UTC のタイムスタンプが、
# マイグレーションを適用する順序を決める。
.PHONY: db-new
db-new: ## Create a new migration file (usage: make db-new name=create_users). [Ja] 新しいマイグレーションファイルを作成
	@if [ -z "$(name)" ]; then \
		echo "Error: set the name variable"; \
		echo "Usage: make db-new name=create_users"; \
		exit 1; \
	fi
	@file="db/migrations/$$(date -u +%Y%m%d%H%M%S)_$(name).sql"; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > "$$file"; \
	echo "Created $$file"

.PHONY: sqlc-generate
sqlc-generate: ## Generate Go code from SQL queries. [Ja] SQL クエリから Go コードを生成
	sqlc generate
