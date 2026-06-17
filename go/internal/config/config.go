// Package config provides loading and access to application settings from
// environment variables.
//
// [Ja] config パッケージは、環境変数からアプリケーション設定を読み込み、
// アクセスする機能を提供します。
package config

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Config holds the application settings.
//
// [Ja] Config はアプリケーションの設定を保持します。
type Config struct {
	// Env is the running environment: "dev", "test", or "prod".
	//
	// [Ja] Env は実行環境 ("dev" / "test" / "prod") を表します。
	Env string

	// DatabaseURL is the PostgreSQL connection string.
	//
	// [Ja] DatabaseURL は PostgreSQL の接続文字列です。
	DatabaseURL string

	// Port is the TCP port the HTTP server listens on.
	//
	// [Ja] Port は HTTP サーバーが待ち受ける TCP ポートです。
	Port string

	// AssetVersion is the cache-busting value (a Git commit hash) used for
	// static assets in non-dev environments. In dev a timestamp is used instead;
	// see GetAssetVersion.
	//
	// [Ja] AssetVersion は非開発環境で静的アセットに使うキャッシュ無効化用の値
	// (Git コミットハッシュ) です。開発環境では代わりにタイムスタンプを使います
	// (GetAssetVersion を参照)。
	AssetVersion string

	// ResendAPIKey, EmailFrom, and EmailFromName configure outgoing email through
	// Resend. They are consumed by the worker client when it builds the email
	// sender for background jobs. They are optional rather than required: the
	// worker is not started yet (its first enqueue-side consumer, sign-up, comes
	// in a later task), so a deployment without email configured must still boot.
	// When the worker is wired into main.go, missing values can be promoted to a
	// startup error then.
	//
	// [Ja] ResendAPIKey / EmailFrom / EmailFromName は Resend 経由の送信メールを設定
	// します。ワーカークライアントがバックグラウンドジョブ用の email sender を構築する
	// 際に使います。必須ではなく任意とします。ワーカーはまだ起動されておらず (最初の
	// 投入側の利用者であるサインアップは後続タスク)、メール未設定のデプロイでも起動できる
	// 必要があるためです。ワーカーを main.go に配線する時点で、欠落を起動時エラーへ
	// 格上げできます。
	ResendAPIKey string

	// EmailFrom is the sender address used in the From header of outgoing email.
	//
	// [Ja] EmailFrom は送信メールの From ヘッダーに使う送信元アドレスです。
	EmailFrom string

	// EmailFromName is the display name shown alongside EmailFrom in the From
	// header.
	//
	// [Ja] EmailFromName は From ヘッダーで EmailFrom と並べて表示する送信元の表示名
	// です。
	EmailFromName string

	// AppURL is the public base URL of the application (e.g.
	// "https://groobb.example.dev" in production, "http://localhost:8080" in
	// dev), with no trailing slash. It is needed to build absolute links in
	// outgoing email (such as the password reset link), which a relative path
	// cannot express. A full base URL is stored rather than a bare domain so it
	// can carry the scheme and port that dev (plain HTTP on a port) requires. It
	// is optional rather than required for the same reason as the email settings
	// (a deployment without email configured must still boot); when the link is
	// built from an empty AppURL the URL is simply host-relative.
	//
	// [Ja] AppURL はアプリケーションの公開ベース URL (例: 本番は
	// "https://groobb.example.dev"、dev は "http://localhost:8080") で、末尾スラッシュは
	// 付けません。送信メール内の絶対リンク (パスワードリセットリンクなど) を組み立てるのに
	// 必要で、相対パスでは表現できません。素のドメインではなくベース URL 全体を保持するのは、
	// dev (ポート上の平文 HTTP) が要求するスキームとポートを含められるようにするためです。
	// メール設定と同じ理由で必須ではなく任意とします (メール未設定のデプロイでも起動できる
	// 必要があるため)。空の AppURL からリンクを組み立てた場合、URL は単にホスト相対になります。
	AppURL string
}

// Load reads the configuration from environment variables.
//
// Every environment expects the variables to be already exported into the
// process before startup: locally `op run --env-file=.env` resolves them, and
// in CI / production the runtime sets them directly.
//
// [Ja] Load は環境変数から設定を読み込みます。
//
// いずれの環境でも、プロセス起動時には環境変数が既にエクスポートされている
// ことを前提とします。ローカルでは `op run --env-file=.env` が解決し、
// CI / 本番ではランタイムが直接設定します。
func Load() (*Config, error) {
	// APP_ENV defaults to "dev" when unset.
	//
	// [Ja] APP_ENV は未設定の場合 "dev" を既定値とします。
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	cfg := &Config{
		Env: env,
	}

	cfg.Port = os.Getenv("GROOBB_PORT")
	if cfg.Port == "" {
		return nil, fmt.Errorf("required environment variable GROOBB_PORT is not set")
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("required environment variable DATABASE_URL is not set")
	}

	// Pin the asset version to the current commit so that non-dev environments
	// serve stable, cache-busting asset URLs per deploy.
	//
	// [Ja] 非開発環境がデプロイ単位で安定したキャッシュ無効化 URL を配信できるよう、
	// アセットバージョンを現在のコミットに固定します。
	cfg.AssetVersion = getGitCommitHash()

	// Email settings are read without requiring them: the worker that uses them
	// is not started yet, so a deployment without email configured must still
	// boot (see the field docs).
	//
	// [Ja] メール設定は必須にせず読み込む。これらを使うワーカーはまだ起動されない
	// ため、メール未設定のデプロイでも起動できる必要がある (フィールドのドキュメントを
	// 参照)。
	cfg.ResendAPIKey = os.Getenv("GROOBB_RESEND_API_KEY")
	cfg.EmailFrom = os.Getenv("GROOBB_EMAIL_FROM")
	cfg.EmailFromName = os.Getenv("GROOBB_EMAIL_FROM_NAME")

	// AppURL is read without requiring it, for the same reason as the email
	// settings above (see the field docs).
	//
	// [Ja] AppURL は必須にせず読み込む。理由は上のメール設定と同じ (フィールドの
	// ドキュメントを参照)。
	cfg.AppURL = os.Getenv("GROOBB_APP_URL")

	return cfg, nil
}

// IsDev reports whether the running environment is development.
//
// [Ja] IsDev は実行環境が開発環境かどうかを返します。
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTest reports whether the running environment is test.
//
// [Ja] IsTest は実行環境がテスト環境かどうかを返します。
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProduction reports whether the running environment is production.
//
// [Ja] IsProduction は実行環境が本番環境かどうかを返します。
func (c *Config) IsProduction() bool {
	return c.Env == "prod"
}

// GetAssetVersion returns the cache-busting value appended to static asset URLs.
//
// In dev it returns a fresh millisecond timestamp on every call so that edits
// to CSS / JS are picked up without manual cache clearing. In other
// environments it returns the static AssetVersion fixed at startup.
//
// [Ja] GetAssetVersion は静的アセットの URL に付与するキャッシュ無効化用の値を
// 返します。
//
// 開発環境では呼び出しごとに新しいミリ秒タイムスタンプを返し、CSS / JS の編集を
// 手動のキャッシュクリアなしに反映できるようにします。それ以外の環境では、起動時に
// 固定した静的な AssetVersion を返します。
func (c *Config) GetAssetVersion() string {
	if c.IsDev() {
		return strconv.FormatInt(time.Now().UnixMilli(), 10)
	}
	return c.AssetVersion
}

// getGitCommitHash returns the short hash of the current Git commit, or "dev"
// as a fallback when Git is unavailable (e.g. a binary running outside a repo).
//
// [Ja] getGitCommitHash は現在の Git コミットの短縮ハッシュを返します。Git が
// 使えない場合 (例: リポジトリ外で動くバイナリ) はフォールバックとして "dev" を
// 返します。
func getGitCommitHash() string {
	out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}
