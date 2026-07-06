package config

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

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

// Cloudflare's documented dummy keys, used only as test fixtures (never as dev
// runtime defaults).
//
// [Ja] Cloudflare が公開しているダミーキー。テストのフィクスチャとしてのみ使い、
// dev の実行時デフォルトには使わない。
const (
	turnstileTestSiteKey   = "1x00000000000000000000AA"
	turnstileTestSecretKey = "1x0000000000000000000000000000000AA"
)

// TestLoadReadsTurnstileSettings verifies the optional Turnstile keys are read
// from the environment, and default to empty when unset (they are not required
// because Turnstile is enabled operationally by provisioning the real keys).
//
// [Ja] TestLoadReadsTurnstileSettings は任意の Turnstile キーが環境変数から読み込まれ、
// 未設定時は空になることを検証する (実キーを設定する運用で有効化するため必須ではない)。
func TestLoadReadsTurnstileSettings(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		setRequiredEnv(t)
		// Keep DISABLE unset so it does not clear the keys under assertion.
		//
		// [Ja] DISABLE を未設定にして、検証対象のキーが空に落とされないようにする。
		t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
		t.Setenv("GROOBB_TURNSTILE_SITE_KEY", turnstileTestSiteKey)
		t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", turnstileTestSecretKey)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() returned an unexpected error: %v", err)
		}

		if cfg.TurnstileSiteKey != turnstileTestSiteKey {
			t.Errorf("TurnstileSiteKey = %q, want %q", cfg.TurnstileSiteKey, turnstileTestSiteKey)
		}
		if cfg.TurnstileSecretKey != turnstileTestSecretKey {
			t.Errorf("TurnstileSecretKey = %q, want %q", cfg.TurnstileSecretKey, turnstileTestSecretKey)
		}
	})

	t.Run("unset defaults to empty without error", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
		t.Setenv("GROOBB_TURNSTILE_SITE_KEY", "")
		t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() should not fail when Turnstile keys are missing: %v", err)
		}

		if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
			t.Errorf("Turnstile keys should default to empty, got %q / %q",
				cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
		}
	})
}

// TestLoadTurnstileDisable verifies the fail-closed disable logic:
// GROOBB_TURNSTILE_DISABLE clears both keys in non-production environments but is
// ignored (keys kept) in production.
//
// [Ja] TestLoadTurnstileDisable は fail-closed の無効化ロジックを検証する。
// GROOBB_TURNSTILE_DISABLE は非本番環境では両キーを空にするが、本番環境では無視され
// (キーを保持する)。
func TestLoadTurnstileDisable(t *testing.T) {
	tests := []struct {
		name            string
		env             string
		disable         string
		wantKeysCleared bool
	}{
		{name: "dev DISABLE clears keys", env: "dev", disable: "true", wantKeysCleared: true},
		{name: "test DISABLE clears keys", env: "test", disable: "true", wantKeysCleared: true},
		{name: "production DISABLE is ignored", env: "prod", disable: "true", wantKeysCleared: false},
		{name: "DISABLE unset keeps keys", env: "test", disable: "", wantKeysCleared: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", tt.env)
			t.Setenv("GROOBB_TURNSTILE_SITE_KEY", turnstileTestSiteKey)
			t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", turnstileTestSecretKey)
			t.Setenv("GROOBB_TURNSTILE_DISABLE", tt.disable)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned an unexpected error: %v", err)
			}

			if tt.wantKeysCleared {
				if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
					t.Errorf("keys should be cleared, got site=%q secret=%q",
						cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
				}
				return
			}

			if cfg.TurnstileSiteKey != turnstileTestSiteKey {
				t.Errorf("TurnstileSiteKey = %q, want %q", cfg.TurnstileSiteKey, turnstileTestSiteKey)
			}
			if cfg.TurnstileSecretKey != turnstileTestSecretKey {
				t.Errorf("TurnstileSecretKey = %q, want %q", cfg.TurnstileSecretKey, turnstileTestSecretKey)
			}
		})
	}
}

// TestLoadTurnstilePartialKeyWarning verifies that Load warns in production when
// exactly one Turnstile key is set (a silent-bypass misconfiguration), and stays
// quiet when both keys are set, both are empty, or the environment is not
// production.
//
// [Ja] TestLoadTurnstilePartialKeyWarning は、本番で Turnstile のキーが片方だけ設定
// されているとき (黙ってバイパスされる設定ミス) に Load が警告し、両方設定・両方空・
// 非本番のときは警告しないことを検証する。
func TestLoadTurnstilePartialKeyWarning(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		siteKey   string
		secretKey string
		wantWarn  bool
	}{
		{name: "prod site key only warns", env: "prod", siteKey: turnstileTestSiteKey, secretKey: "", wantWarn: true},
		{name: "prod secret key only warns", env: "prod", siteKey: "", secretKey: turnstileTestSecretKey, wantWarn: true},
		{name: "prod both keys set does not warn", env: "prod", siteKey: turnstileTestSiteKey, secretKey: turnstileTestSecretKey, wantWarn: false},
		{name: "prod both keys empty does not warn", env: "prod", siteKey: "", secretKey: "", wantWarn: false},
		{name: "dev site key only does not warn", env: "dev", siteKey: turnstileTestSiteKey, secretKey: "", wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", tt.env)
			t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
			t.Setenv("GROOBB_TURNSTILE_SITE_KEY", tt.siteKey)
			t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", tt.secretKey)

			// Capture the default slog output for the duration of the test and
			// restore it afterward, so the partial-key warning can be asserted.
			//
			// [Ja] 片方キー警告を検証できるよう、テスト中だけデフォルトの slog 出力を
			// 捕捉し、終了後に元へ戻す。
			var buf bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
			defer slog.SetDefault(original)

			if _, err := Load(); err != nil {
				t.Fatalf("Load() returned an unexpected error: %v", err)
			}

			warned := strings.Contains(buf.String(), "片方のみ設定")
			if warned != tt.wantWarn {
				t.Errorf("partial-key warning logged = %v, want %v (log: %q)", warned, tt.wantWarn, buf.String())
			}
		})
	}
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
