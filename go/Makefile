# CIでは環境変数が既にエクスポート済みのためop runを介しません。
# ローカルでは .env内のop:// 参照をop runが解決してから起動します。
ifeq ($(CI),true)
  OP_RUN_DEV = APP_ENV=dev
else
  OP_RUN_DEV = APP_ENV=dev op run --env-file=".env" --
endif

.PHONY: help
help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## HTTPサーバーを起動
	$(OP_RUN_DEV) go run ./cmd/groobb serve

.PHONY: browse-login
browse-login: ## devサイトへブラウザでログイン (別アカウントはmake browse-login BROWSE_USER=replier)
	$(OP_RUN_DEV) bash scripts/browse.sh login $(BROWSE_USER)

.PHONY: browse-shot
browse-shot: ## ログイン済みセッションでページを開きスクショ (例: make browse-shot URL=/)
	bash scripts/browse.sh shot "$(URL)"

.PHONY: browse-close
browse-close: ## ブラウザセッションを閉じ、資格情報・originファイル・profileを削除 (スクショは残す)
	bash scripts/browse.sh close

.PHONY: assets
assets: ## CSS / JSをstatic/ にビルド
	pnpm build

# バイナリはコンパイル時点のstatic/ の内容を埋め込むため、リリースは
# `make assets build` でビルドする。build単体はコンパイルの確認であり、アセットは
# そのままにする。
#
# アセットバージョンをここで埋め込むのは、サーバーが描画するURLがそれを伴う一方、
# 配布されたバイナリの周囲には実行時にリビジョンを読めるリポジトリが無いためである。
# 埋め込みが無い場合、ビルドはツールチェインが記録したリビジョンへフォールバックする
# (internal/configのbuildAssetVersionを参照)。
# 両方の経路で完全なリビジョンを使い、短縮した接頭辞の衝突によって異なるコミットが
# 同じ長期キャッシュキーを共有しないようにする。
ASSET_VERSION ?= $(shell git rev-parse HEAD 2>/dev/null || echo dev)
ASSET_VERSION_LDFLAGS = -X github.com/groobb/groobb/go/internal/config.assetVersion=$(ASSET_VERSION)

.PHONY: build
build: ## groobbバイナリをbin/groobbにビルド
	go build -ldflags "$(ASSET_VERSION_LDFLAGS)" -o bin/groobb ./cmd/groobb

.PHONY: templ-generate
templ-generate: ## templファイルからGoコードを生成 (goimportsも実行)
	go tool templ generate
	@$(MAKE) goimports

.PHONY: fmt
fmt: ## Goコードをフォーマット (go fmt + goimports)
	go fmt ./...
	@$(MAKE) goimports

.PHONY: goimports
goimports: ## goimportsでimport文を整理
	go tool goimports -w .

.PHONY: goimports-check
goimports-check: ## goimportsでimport文をチェック (差分があれば失敗)
	@unformatted=$$(go tool goimports -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "次のファイルのimport文が整理されていません:"; \
		echo "$$unformatted"; \
		echo ""; \
		echo "make goimportsを実行してimport文を整理してください"; \
		exit 1; \
	fi

.PHONY: lint
lint: lint-style ## golangci-lintと日本語テキストのスタイルチェックを実行
	golangci-lint run --config=.golangci.yml ./...

# koryluslintはkorylus-toolsが提供するKorylus共通リンタ。
# その `comment` サブコマンドは、コメント・設定ファイル・SQL・シェルなどの日本語テキストがkorylus-lang.md §3のスタイルに沿っているかを検査する。
# バージョンはgo.modのtoolディレクティブで固定し、`go tool koryluslint` がgo/を作業ディレクトリとして固定版を実行する。
# Tree-sitterのGoバインディングを使うため、ビルドにCコンパイラと `CGO_ENABLED=1` が要る。
#
# CIと `lint` はツリー全体を検査する `lint-style` を使う。
# `lint-style-base` は、ブランチで追加した行だけを手元で確かめたいときに使う。

.PHONY: lint-style
lint-style: ## 日本語テキストのスタイルをツリー全体でチェック (koryluslint)
	@go tool koryluslint comment

.PHONY: lint-style-base
lint-style-base: ## BASE refとの差分行で日本語テキストのスタイルをチェック (例: BASE=origin/develop)
	@go tool koryluslint comment -base=$(BASE)

# マイグレーション済みのSQLiteスキーマスナップショットをテストバイナリごとに
# 1度だけ作り、各テスト専用のデータベースファイルへ複製するため、テストスイートは
# 事前に用意されたデータベースも環境変数も必要とせず、opを介さずに走る。
# tparseは -jsonストリームを整形するツールで、go.modのtoolディレクティブ経由で
# 導入し、ここでの他の開発ツールと同じ呼び出し方に揃えている。set -o pipefailは
# go testの失敗をtparseパイプ越しに伝播させる。これが無いとmakeはtparseの
# 終了コードしか見ず、失敗した実行を成功と誤認してしまう。
.PHONY: test
test: ## テストを実行
	@bash -c 'set -o pipefail && go test -json -race ./... | go tool tparse -all'

.PHONY: test-pkg
test-pkg: ## 特定パッケージのテストを実行 (例: make test-pkg PKG=internal/handler/sign_in)
	@if [ -z "$(PKG)" ]; then \
		echo "エラー: PKG変数を指定してください"; \
		echo "使い方: make test-pkg PKG=internal/handler/sign_in"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && go test -json -race ./$(PKG) | go tool tparse -all'

.PHONY: test-run
test-run: ## 特定のテストを実行 (例: make test-run PKG=internal/handler/sign_in RUN=TestCreate)
	@if [ -z "$(PKG)" ]; then \
		echo "エラー: PKG変数を指定してください"; \
		echo "使い方: make test-run PKG=internal/handler/sign_in RUN=TestCreate"; \
		exit 1; \
	fi
	@if [ -z "$(RUN)" ]; then \
		echo "エラー: RUN変数を指定してください"; \
		echo "使い方: make test-run PKG=internal/handler/sign_in RUN=TestCreate"; \
		exit 1; \
	fi
	@bash -c 'set -o pipefail && go test -json -race ./$(PKG) -run $(RUN) | go tool tparse -all'

.PHONY: test-verbose
test-verbose: ## 詳細ログ付きで全テストを実行 (デバッグ用)
	@bash -c 'set -o pipefail && go test -json -race -count=1 ./... | go tool tparse -all'

# データベースコマンド。db-migrate / db-rollbackはデータベースのパスを .env
# から読み、op runがコマンドの起動前にそれを読み込む。db-newはファイルを作るだけの
# ため、どちらも必要としない。

.PHONY: db-migrate
db-migrate: ## 未適用のマイグレーションを実行
	$(OP_RUN_DEV) go run ./cmd/groobb migrate up

.PHONY: db-rollback
db-rollback: ## 直近のマイグレーションをロールバック
	$(OP_RUN_DEV) go run ./cmd/groobb migrate down

# 生成されるファイルは、gooseがマイグレーションをupとdownの2つに分けて読む
# ための注釈2行を持つ。ファイル名の先頭に付けるUTCのタイムスタンプが、
# マイグレーションを適用する順序を決める。
.PHONY: db-new
db-new: ## 新しいマイグレーションファイルを作成 (例: make db-new name=create_users)
	@if [ -z "$(name)" ]; then \
		echo "エラー: name変数を指定してください"; \
		echo "使い方: make db-new name=create_users"; \
		exit 1; \
	fi
	@file="db/migrations/$$(date -u +%Y%m%d%H%M%S)_$(name).sql"; \
	printf -- '-- +goose Up\n\n-- +goose Down\n' > "$$file"; \
	echo "$${file}を作成しました"

# seedは開発用データベースを、その画面を見るために使うデータで作り直す。
# データベースのパスは .envから読み、op runがコマンドの起動前にそれを読み込む。
# アカウントの名簿は作業ディレクトリから読むため、上のコマンドと同じくGoモジュールの
# ルートで実行する。
#
# profileは生成するコミュニティの状態を選ぶもので、既定は成熟した状態である。
# `make seed profile=cold-start` はインスタンスが開くときの状態を作る。行数の少ない画面を
# 確かめる先はこちらになる (ADR 0010)。何も書かれていない名前を渡した実行は、何かを削除
# する前に、受け付ける状態を挙げて終わる。
.PHONY: seed
seed: ## 開発用データベースをシードデータで作り直す (例: make seed profile=cold-start)
	$(OP_RUN_DEV) go run ./cmd/groobb seed $(profile)

.PHONY: sqlc-generate
sqlc-generate: ## SQLクエリからGoコードを生成
	go tool sqlc generate

# `sqlc diff` は再生成と同じずれを、作業ツリーへ書き込まずに報告する。
# 検査が落ちても、作業中のブランチに意図しない差分が残らない。生成はsqlc-generate
# の責務のままとする。
.PHONY: sqlc-diff
sqlc-diff: ## 生成コードがSQLと同期しているかチェック (差分があれば失敗)
	go tool sqlc diff
