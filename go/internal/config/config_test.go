package config

import "testing"

// setRequiredEnv sets the environment variables that Load requires.
// t.Setenv restores the previous values automatically when the test ends.
//
// [Ja] setRequiredEnv は Load が必須とする環境変数を設定します。
// t.Setenv はテスト終了時に元の値を自動的に復元します。
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("APP_ENV", "test")
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("DATABASE_URL", "postgres://postgres@localhost:5432/groobb_test?sslmode=disable")
}

// TestLoad verifies that Load reads the required environment variables.
//
// [Ja] TestLoad は Load が必須の環境変数を読み込むことを検証します。
func TestLoad(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.Env != "test" {
		t.Errorf("Env = %q, want %q", cfg.Env, "test")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL should not be empty")
	}
}

// TestLoadReadsEmailSettings verifies the optional Resend email settings are
// read from the environment, and default to empty when unset (they are not
// required because the worker that uses them is not started yet).
//
// [Ja] TestLoadReadsEmailSettings は任意の Resend メール設定が環境変数から読み込まれ、
// 未設定時は空になることを検証する (これらを使うワーカーはまだ起動されないため必須では
// ない)。
func TestLoadReadsEmailSettings(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_RESEND_API_KEY", "re_test_key")
		t.Setenv("GROOBB_EMAIL_FROM", "noreply@example.dev")
		t.Setenv("GROOBB_EMAIL_FROM_NAME", "Groobb")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned an unexpected error: %v", err)
		}

		if cfg.ResendAPIKey != "re_test_key" {
			t.Errorf("ResendAPIKey = %q, want %q", cfg.ResendAPIKey, "re_test_key")
		}
		if cfg.EmailFrom != "noreply@example.dev" {
			t.Errorf("EmailFrom = %q, want %q", cfg.EmailFrom, "noreply@example.dev")
		}
		if cfg.EmailFromName != "Groobb" {
			t.Errorf("EmailFromName = %q, want %q", cfg.EmailFromName, "Groobb")
		}
	})

	t.Run("unset defaults to empty without error", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_RESEND_API_KEY", "")
		t.Setenv("GROOBB_EMAIL_FROM", "")
		t.Setenv("GROOBB_EMAIL_FROM_NAME", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() should not fail when email settings are missing: %v", err)
		}

		if cfg.ResendAPIKey != "" || cfg.EmailFrom != "" || cfg.EmailFromName != "" {
			t.Errorf("email settings should default to empty, got %q / %q / %q",
				cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailFromName)
		}
	})
}

// TestLoadReadsAppURL verifies the optional AppURL is read from the environment,
// and defaults to empty when unset (it is not required for the same reason as
// the email settings: a deployment without it must still boot).
//
// [Ja] TestLoadReadsAppURL は任意の AppURL が環境変数から読み込まれ、未設定時は空に
// なることを検証する (メール設定と同じ理由で必須ではない: 未設定でも起動できる必要がある)。
func TestLoadReadsAppURL(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_APP_URL", "https://groobb.example.dev")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned an unexpected error: %v", err)
		}

		if cfg.AppURL != "https://groobb.example.dev" {
			t.Errorf("AppURL = %q, want %q", cfg.AppURL, "https://groobb.example.dev")
		}
	})

	t.Run("unset defaults to empty without error", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_APP_URL", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() should not fail when AppURL is missing: %v", err)
		}

		if cfg.AppURL != "" {
			t.Errorf("AppURL should default to empty, got %q", cfg.AppURL)
		}
	})
}

// TestLoadDefaultsEnvToDev verifies that an empty APP_ENV defaults to "dev".
//
// [Ja] TestLoadDefaultsEnvToDev は APP_ENV が空のとき "dev" が既定値になることを検証します。
func TestLoadDefaultsEnvToDev(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.Env != "dev" {
		t.Errorf("Env = %q, want %q", cfg.Env, "dev")
	}
}

// TestLoadMissingRequiredEnv verifies that Load fails when a required variable is missing.
//
// [Ja] TestLoadMissingRequiredEnv は必須の環境変数が欠けているとき Load が失敗することを検証します。
func TestLoadMissingRequiredEnv(t *testing.T) {
	tests := []struct {
		name  string
		unset string
	}{
		{name: "GROOBB_PORT is missing", unset: "GROOBB_PORT"},
		{name: "DATABASE_URL is missing", unset: "DATABASE_URL"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(tt.unset, "")

			if _, err := Load(); err == nil {
				t.Errorf("Load() should fail when %s is missing, but got nil error", tt.unset)
			}
		})
	}
}

// TestEnvHelpers verifies the IsDev / IsTest / IsProduction helpers.
//
// [Ja] TestEnvHelpers は IsDev / IsTest / IsProduction ヘルパーを検証します。
func TestEnvHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		env          string
		isDev        bool
		isTest       bool
		isProduction bool
	}{
		{env: "dev", isDev: true},
		{env: "test", isTest: true},
		{env: "prod", isProduction: true},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := &Config{Env: tt.env}
			if got := cfg.IsDev(); got != tt.isDev {
				t.Errorf("IsDev() = %v, want %v", got, tt.isDev)
			}
			if got := cfg.IsTest(); got != tt.isTest {
				t.Errorf("IsTest() = %v, want %v", got, tt.isTest)
			}
			if got := cfg.IsProduction(); got != tt.isProduction {
				t.Errorf("IsProduction() = %v, want %v", got, tt.isProduction)
			}
		})
	}
}

// TestGetAssetVersion verifies that dev returns a non-empty dynamic value and
// that other environments return the static AssetVersion fixed at startup.
//
// [Ja] TestGetAssetVersion は、開発環境では空でない動的な値を返し、それ以外の
// 環境では起動時に固定した静的な AssetVersion を返すことを検証します。
func TestGetAssetVersion(t *testing.T) {
	t.Parallel()

	t.Run("dev returns a non-empty dynamic value", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Env: "dev"}
		if got := cfg.GetAssetVersion(); got == "" {
			t.Error("GetAssetVersion() should not be empty in dev")
		}
	})

	t.Run("non-dev returns the static AssetVersion", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Env: "prod", AssetVersion: "abc123"}
		if got := cfg.GetAssetVersion(); got != "abc123" {
			t.Errorf("GetAssetVersion() = %q, want %q", got, "abc123")
		}
	})
}
