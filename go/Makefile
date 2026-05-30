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

.PHONY: test
test: ## Run tests. [Ja] テストを実行
	go test -race ./...
