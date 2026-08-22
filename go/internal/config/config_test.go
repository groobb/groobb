package config

import (
	"bytes"
	"log/slog"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/email"
)

// clearEnv moves the test into an empty working directory and unsets every
// setting in the environment, so that a case states exactly which source
// provides which value. Without it a case would read whatever the developer's
// own environment holds, and any configuration file that happens to sit in the
// package directory.
//
// [Ja] clearEnv はテストを空の作業ディレクトリへ移し、環境変数の設定をすべて未設定に
// します。各ケースがどの入力からどの値を得るかを明示するためです。これが無いと、ケースは
// 開発者自身の環境変数や、パッケージのディレクトリにたまたま置かれた設定ファイルを
// 読んでしまいます。
func clearEnv(t *testing.T) {
	t.Helper()

	t.Chdir(t.TempDir())

	for _, name := range []string{
		configFileEnvName,
		"APP_ENV",
		"GROOBB_PORT",
		"GROOBB_DATABASE_PATH",
		"GROOBB_CONTINUATION_TOKEN_KEY",
		"GROOBB_APP_URL",
		"GROOBB_EMAIL_PROVIDER",
		"GROOBB_EMAIL_FROM",
		"GROOBB_EMAIL_FROM_NAME",
		"GROOBB_RESEND_API_KEY",
		"GROOBB_SMTP_HOST",
		"GROOBB_SMTP_PORT",
		"GROOBB_SMTP_USERNAME",
		"GROOBB_SMTP_PASSWORD",
		"GROOBB_SMTP_TLS_MODE",
		"GROOBB_TURNSTILE_SITE_KEY",
		"GROOBB_TURNSTILE_SECRET_KEY",
		"GROOBB_TURNSTILE_DISABLE",
	} {
		t.Setenv(name, "")
	}
}

// setRequiredEnv sets the environment variables that Load requires, from a
// cleared environment.
// t.Setenv restores the previous values automatically when the test ends.
//
// [Ja] setRequiredEnv は、クリアされた環境変数の状態から、Load が必須とする環境変数を
// 設定します。
// t.Setenv はテスト終了時に元の値を自動的に復元します。
func setRequiredEnv(t *testing.T) {
	t.Helper()
	clearEnv(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_DATABASE_PATH", "tmp/groobb_test.sqlite")
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
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
	if cfg.DatabasePath != "tmp/groobb_test.sqlite" {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "tmp/groobb_test.sqlite")
	}
	if cfg.ContinuationTokenKey != "groobb-test-continuation-token-key-32-bytes" {
		t.Errorf("ContinuationTokenKey was not loaded from the environment")
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
		{name: "GROOBB_DATABASE_PATH is missing", unset: "GROOBB_DATABASE_PATH"},
		{name: "GROOBB_CONTINUATION_TOKEN_KEY is missing", unset: "GROOBB_CONTINUATION_TOKEN_KEY"},
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

// TestLoadRejectsShortContinuationTokenKey verifies that an easily guessed key
// cannot reach the signing code through the normal application startup path.
//
// [Ja] TestLoadRejectsShortContinuationTokenKey は、推測しやすい短い鍵が通常のアプリ起動
// 経路から署名処理へ到達できないことを検証します。
func TestLoadRejectsShortContinuationTokenKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should fail when GROOBB_CONTINUATION_TOKEN_KEY is shorter than 32 bytes")
	}
}

// TestLoadRejectsAnInvalidPort verifies that a port the server cannot listen on
// stops startup naming the setting, rather than surfacing as a failure to bind
// or, for 0, as an instance listening on whatever port the kernel handed out.
//
// [Ja] TestLoadRejectsAnInvalidPort は、サーバーが待ち受けられないポートが設定名を挙げて
// 起動を止めることを検証する。bind の失敗として現れたり、0 の場合にカーネルが割り当てた
// ポートで待ち受けるインスタンスになったりしないようにするため。
func TestLoadRejectsAnInvalidPort(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "not a number", value: "http"},
		{name: "out of range", value: "70000"},
		{name: "zero", value: "0"},
		{name: "negative", value: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_PORT", tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load() should reject GROOBB_PORT = %q", tt.value)
			}
			if !strings.Contains(err.Error(), "GROOBB_PORT") {
				t.Errorf("the error should name the source of the value, got: %v", err)
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

// TestBuildAssetVersion verifies the order the asset version falls back in, so
// that a build without the stamp still serves a value that changes per revision
// and only a build with neither settles on the fixed placeholder.
//
// [Ja] TestBuildAssetVersion はアセットバージョンのフォールバック順序を検証します。
// 埋め込みの無いビルドでもリビジョンごとに変わる値を配信し、どちらも無いビルドだけが
// 固定のプレースホルダーに落ち着くようにするためです。
func TestBuildAssetVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stamped  string
		revision string
		want     string
	}{
		{
			name:     "prefers the value stamped in at build time",
			stamped:  "19ae8301290f4dc0e814bd0298d9e5c73cda684c",
			revision: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
			want:     "19ae8301290f4dc0e814bd0298d9e5c73cda684c",
		},
		{
			name:     "falls back to the build revision",
			stamped:  "",
			revision: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
			want:     "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
		},
		{
			name:     "falls back to dev without either",
			stamped:  "",
			revision: "",
			want:     "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := buildAssetVersion(tt.stamped, tt.revision); got != tt.want {
				t.Errorf("buildAssetVersion(%q, %q) = %q, want %q", tt.stamped, tt.revision, got, tt.want)
			}
		})
	}
}

// TestVCSRevisionFromSettings verifies revision extraction independently of the
// build information carried by the test binary, which normally has no VCS
// settings.
//
// [Ja] TestVCSRevisionFromSettings は、通常 VCS 設定を持たないテストバイナリ自身の
// ビルド情報に依存せず、リビジョンの抽出を検証します。
func TestVCSRevisionFromSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "returns a full revision unchanged",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6"},
			},
			want: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
		},
		{
			name: "keeps a short revision",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
			},
			want: "abc123",
		},
		{
			name: "returns empty without a revision setting",
			settings: []debug.BuildSetting{
				{Key: "vcs.modified", Value: "true"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := vcsRevisionFromSettings(tt.settings); got != tt.want {
				t.Errorf("vcsRevisionFromSettings(%v) = %q, want %q", tt.settings, got, tt.want)
			}
		})
	}
}

// TestLoadEmailProviderDefaultsToResend verifies an unset provider keeps the
// Resend transport, so a deployment made before the setting existed is unchanged.
//
// [Ja] TestLoadEmailProviderDefaultsToResend は、プロバイダー未設定のとき Resend の
// transport が維持されることを検証する。この設定が存在する前のデプロイが変わらないため。
func TestLoadEmailProviderDefaultsToResend(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.EmailProvider != EmailProviderResend {
		t.Errorf("EmailProvider = %q, want %q", cfg.EmailProvider, EmailProviderResend)
	}
}

// TestLoadRejectsUnknownEmailProvider verifies a provider outside the supported
// set stops startup rather than silently falling back to one of them.
//
// [Ja] TestLoadRejectsUnknownEmailProvider は、対応していないプロバイダーが指定された
// とき、いずれかへ黙ってフォールバックせず起動を止めることを検証する。
func TestLoadRejectsUnknownEmailProvider(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GROOBB_EMAIL_PROVIDER", "sendmail")

	if _, err := Load(); err == nil {
		t.Fatal("Load() should reject an unknown GROOBB_EMAIL_PROVIDER")
	}
}

// setSMTPEnv sets a complete, valid set of SMTP settings that individual cases
// then break in one place.
//
// [Ja] setSMTPEnv は妥当で完全な SMTP 設定一式を設定する。各ケースはそこから 1 箇所だけを
// 壊す。
func setSMTPEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GROOBB_EMAIL_PROVIDER", EmailProviderSMTP)
	t.Setenv("GROOBB_EMAIL_FROM", "noreply@example.dev")
	t.Setenv("GROOBB_SMTP_HOST", "smtp.example.dev")
	t.Setenv("GROOBB_SMTP_PORT", "587")
	t.Setenv("GROOBB_SMTP_USERNAME", "smtp-user")
	t.Setenv("GROOBB_SMTP_PASSWORD", "smtp-password")
	t.Setenv("GROOBB_SMTP_TLS_MODE", smtpTLSModeStartTLS)
}

// TestLoadReadsSMTPSettings verifies the relay settings are read into the
// configuration when the SMTP provider is selected.
//
// [Ja] TestLoadReadsSMTPSettings は、SMTP プロバイダーが選択されたときにリレーの設定が
// 設定へ読み込まれることを検証する。
func TestLoadReadsSMTPSettings(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.EmailProvider != EmailProviderSMTP {
		t.Errorf("EmailProvider = %q, want %q", cfg.EmailProvider, EmailProviderSMTP)
	}
	if cfg.SMTPHost != "smtp.example.dev" {
		t.Errorf("SMTPHost = %q, want %q", cfg.SMTPHost, "smtp.example.dev")
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d, want 587", cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "smtp-user" {
		t.Errorf("SMTPUsername = %q, want %q", cfg.SMTPUsername, "smtp-user")
	}
	if cfg.SMTPPassword != "smtp-password" {
		t.Errorf("SMTPPassword was not loaded from the environment")
	}
	if cfg.SMTPTLSMode != smtpTLSModeStartTLS {
		t.Errorf("SMTPTLSMode = %q, want %q", cfg.SMTPTLSMode, smtpTLSModeStartTLS)
	}
}

// TestLoadDefaultsSMTPTLSModeToStartTLS verifies an unset TLS mode secures the
// connection rather than leaving it in the clear.
//
// [Ja] TestLoadDefaultsSMTPTLSModeToStartTLS は、TLS モード未設定のときに接続を平文の
// ままにせず保護することを検証する。
func TestLoadDefaultsSMTPTLSModeToStartTLS(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)
	t.Setenv("GROOBB_SMTP_TLS_MODE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.SMTPTLSMode != smtpTLSModeStartTLS {
		t.Errorf("SMTPTLSMode = %q, want %q", cfg.SMTPTLSMode, smtpTLSModeStartTLS)
	}
}

// TestLoadAcceptsSMTPTLSModes fixes every supported mode at the environment
// boundary so none can disappear from startup configuration while the sender's
// direct tests continue to pass.
//
// [Ja] TestLoadAcceptsSMTPTLSModes は、サポートするすべてのモードを環境変数との
// 境界で固定する。Sender の直接テストが通り続ける一方で、起動設定からいずれかの
// モードが欠落することを防ぐため。
func TestLoadAcceptsSMTPTLSModes(t *testing.T) {
	tests := []string{
		smtpTLSModeStartTLS,
		smtpTLSModeImplicit,
		smtpTLSModeNone,
	}

	for _, mode := range tests {
		t.Run(mode, func(t *testing.T) {
			setRequiredEnv(t)
			setSMTPEnv(t)
			t.Setenv("GROOBB_SMTP_TLS_MODE", mode)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() returned an unexpected error: %v", err)
			}

			if cfg.SMTPTLSMode != mode {
				t.Errorf("SMTPTLSMode = %q, want %q", cfg.SMTPTLSMode, mode)
			}
		})
	}
}

// TestLoadAcceptsSMTPWithoutCredentials verifies a relay that authorises by
// source address is a valid configuration.
//
// [Ja] TestLoadAcceptsSMTPWithoutCredentials は、送信元アドレスで認可するリレーが妥当な
// 設定であることを検証する。
func TestLoadAcceptsSMTPWithoutCredentials(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)
	t.Setenv("GROOBB_SMTP_USERNAME", "")
	t.Setenv("GROOBB_SMTP_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if cfg.SMTPUsername != "" || cfg.SMTPPassword != "" {
		t.Error("credentials should stay empty when neither variable is set")
	}
}

// TestLoadRejectsIncompleteSMTPSettings checks each way the relay settings can be
// wrong, so an operator gets a startup error instead of mail that never arrives.
//
// [Ja] TestLoadRejectsIncompleteSMTPSettings はリレー設定が誤りうる各ケースを確認する。
// 運用者が、届かないメールではなく起動時エラーを受け取るようにするため。
func TestLoadRejectsIncompleteSMTPSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "host missing", key: "GROOBB_SMTP_HOST", value: ""},
		{name: "port missing", key: "GROOBB_SMTP_PORT", value: ""},
		{name: "port not a number", key: "GROOBB_SMTP_PORT", value: "submission"},
		{name: "port out of range", key: "GROOBB_SMTP_PORT", value: "70000"},
		{name: "port zero", key: "GROOBB_SMTP_PORT", value: "0"},
		{name: "username without password", key: "GROOBB_SMTP_PASSWORD", value: ""},
		{name: "password without username", key: "GROOBB_SMTP_USERNAME", value: ""},
		{name: "unknown TLS mode", key: "GROOBB_SMTP_TLS_MODE", value: "ssl"},
		{name: "From address missing", key: "GROOBB_EMAIL_FROM", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			setSMTPEnv(t)
			t.Setenv(tt.key, tt.value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load() should reject %s = %q", tt.key, tt.value)
			}
		})
	}
}

// TestLoadWarnsOnUnencryptedSMTPInProduction verifies the unencrypted relay is
// allowed but called out, since it is only safe on a trusted local channel.
//
// [Ja] TestLoadWarnsOnUnencryptedSMTPInProduction は、暗号化しないリレーが許容されつつ
// 指摘されることを検証する。安全なのは信頼できるローカル経路に限られるため。
func TestLoadWarnsOnUnencryptedSMTPInProduction(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)
	t.Setenv("APP_ENV", "prod")
	t.Setenv("GROOBB_SMTP_TLS_MODE", smtpTLSModeNone)

	var buf bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(original) })

	if _, err := Load(); err != nil {
		t.Fatalf("Load() returned an unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "SMTP") {
		t.Errorf("expected a warning about the unencrypted relay, got: %s", buf.String())
	}
}

// TestSMTPTLSModeValuesMatchEmailPackage pins the literals this package repeats
// to the email package's constants. Only the worker's unchecked string
// conversion joins the two at runtime, so a value that drifts on one side would
// still load here and then fall through to the sender's STARTTLS default.
//
// The import is test-only: the production config package avoids importing the
// email package, which would pull in the mail templates while config is imported
// by nearly every package.
//
// [Ja] TestSMTPTLSModeValuesMatchEmailPackage は、本パッケージが再掲しているリテラルを
// email パッケージの定数に固定する。実行時に両者を繋ぐのはワーカーの検査を伴わない文字列
// 変換だけなので、片側の値がずれてもここでは読み込めてしまい、Sender 側では STARTTLS の
// 既定へ落ちてしまう。
//
// この import はテスト専用であり、本番の config パッケージが email パッケージを import
// しないようにする。email パッケージはメールテンプレートを引き込む一方、config はほぼ
// すべてのパッケージから import されるためである。
func TestSMTPTLSModeValuesMatchEmailPackage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want email.SMTPTLSMode
	}{
		{name: "starttls", got: smtpTLSModeStartTLS, want: email.SMTPTLSModeStartTLS},
		{name: "implicit", got: smtpTLSModeImplicit, want: email.SMTPTLSModeImplicit},
		{name: "none", got: smtpTLSModeNone, want: email.SMTPTLSModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.got != string(tt.want) {
				t.Errorf("config value = %q, want %q from the email package", tt.got, tt.want)
			}
		})
	}
}

// TestLogValueRedactsSecrets verifies that logging a Config keeps the values
// that authenticate the instance out of the log, while still showing which
// secrets are set and every setting that is not one.
//
// [Ja] TestLogValueRedactsSecrets は、Config をログに出してもインスタンスの認証に使う値が
// ログへ入らないこと、その一方でどの秘密情報が設定されているかと、秘密情報でない設定は
// 見えることを検証します。
func TestLogValueRedactsSecrets(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Env:                  "prod",
		SMTPHost:             "smtp.example.dev",
		ContinuationTokenKey: "continuation-token-key-that-must-not-be-logged",
		ResendAPIKey:         "resend-api-key-that-must-not-be-logged",
		SMTPPassword:         "smtp-password-that-must-not-be-logged",
		TurnstileSecretKey:   "turnstile-secret-key-that-must-not-be-logged",
	}

	tests := []struct {
		name  string
		value any
	}{
		{name: "pointer", value: &cfg},
		{name: "value", value: cfg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			slog.New(slog.NewTextHandler(&buf, nil)).Info("configuration", "config", tt.value)
			logged := buf.String()

			secrets := []string{
				cfg.ContinuationTokenKey,
				cfg.ResendAPIKey,
				cfg.SMTPPassword,
				cfg.TurnstileSecretKey,
			}
			for _, secret := range secrets {
				if strings.Contains(logged, secret) {
					t.Errorf("the log should not hold the secret %q, got: %s", secret, logged)
				}
			}

			if !strings.Contains(logged, redactedSecret) {
				t.Errorf("the log should mark the secrets that are set, got: %s", logged)
			}
			if !strings.Contains(logged, cfg.SMTPHost) {
				t.Errorf("the log should keep the settings that are not secrets, got: %s", logged)
			}
		})
	}
}
