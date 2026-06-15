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
