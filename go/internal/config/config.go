// Package config provides loading and access to application settings from
// environment variables.
//
// [Ja] config パッケージは、環境変数からアプリケーション設定を読み込み、
// アクセスする機能を提供します。
package config

import (
	"fmt"
	"os"
)

// Config holds the application settings.
// [Ja] Config はアプリケーションの設定を保持します。
type Config struct {
	// Env is the running environment: "dev", "test", or "prod".
	// [Ja] Env は実行環境 ("dev" / "test" / "prod") を表します。
	Env string

	// DatabaseURL is the PostgreSQL connection string.
	// [Ja] DatabaseURL は PostgreSQL の接続文字列です。
	DatabaseURL string

	// Port is the TCP port the HTTP server listens on.
	// [Ja] Port は HTTP サーバーが待ち受ける TCP ポートです。
	Port string
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

	return cfg, nil
}

// IsDev reports whether the running environment is development.
// [Ja] IsDev は実行環境が開発環境かどうかを返します。
func (c *Config) IsDev() bool {
	return c.Env == "dev"
}

// IsTest reports whether the running environment is test.
// [Ja] IsTest は実行環境がテスト環境かどうかを返します。
func (c *Config) IsTest() bool {
	return c.Env == "test"
}

// IsProduction reports whether the running environment is production.
// [Ja] IsProduction は実行環境が本番環境かどうかを返します。
func (c *Config) IsProduction() bool {
	return c.Env == "prod"
}
