package config

import (
	"bytes"
	"log/slog"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/email"
)

// clearEnvはテストを空の作業ディレクトリへ移し、環境変数の設定をすべて未設定に
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
		"GROOBB_TRUSTED_PROXIES",
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

// setRequiredEnvは、クリアされた環境変数の状態から、Loadが必須とする環境変数を
// 設定します。
// t.Setenvはテスト終了時に元の値を自動的に復元します。
func setRequiredEnv(t *testing.T) {
	t.Helper()
	clearEnv(t)
	t.Setenv("APP_ENV", "test")
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_DATABASE_PATH", "tmp/groobb_test.sqlite")
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
}

// TestLoadはLoadが必須の環境変数を読み込むことを検証します。
func TestLoad(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.Env != "test" {
		t.Errorf("Env = %q、期待値 = %q", cfg.Env, "test")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q、期待値 = %q", cfg.Port, "8080")
	}
	if cfg.DatabasePath != "tmp/groobb_test.sqlite" {
		t.Errorf("DatabasePath = %q、期待値 = %q", cfg.DatabasePath, "tmp/groobb_test.sqlite")
	}
	if cfg.ContinuationTokenKey != "groobb-test-continuation-token-key-32-bytes" {
		t.Errorf("ContinuationTokenKeyが環境変数から読み込まれていない")
	}
}

// TestLoadReadsEmailSettingsは任意のResendメール設定が環境変数から読み込まれ、
// 未設定時は空になることを検証する (これらを使うワーカーはまだ起動されないため必須では
// ない)。
func TestLoadReadsEmailSettings(t *testing.T) {
	t.Run("設定あり", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_RESEND_API_KEY", "re_test_key")
		t.Setenv("GROOBB_EMAIL_FROM", "noreply@example.dev")
		t.Setenv("GROOBB_EMAIL_FROM_NAME", "Groobb")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load()が予期しないエラーを返した: %v", err)
		}

		if cfg.ResendAPIKey != "re_test_key" {
			t.Errorf("ResendAPIKey = %q、期待値 = %q", cfg.ResendAPIKey, "re_test_key")
		}
		if cfg.EmailFrom != "noreply@example.dev" {
			t.Errorf("EmailFrom = %q、期待値 = %q", cfg.EmailFrom, "noreply@example.dev")
		}
		if cfg.EmailFromName != "Groobb" {
			t.Errorf("EmailFromName = %q、期待値 = %q", cfg.EmailFromName, "Groobb")
		}
	})

	t.Run("未設定ならエラー無しで空になる", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_RESEND_API_KEY", "")
		t.Setenv("GROOBB_EMAIL_FROM", "")
		t.Setenv("GROOBB_EMAIL_FROM_NAME", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("メール設定が無いときにLoad()が失敗した: %v", err)
		}

		if cfg.ResendAPIKey != "" || cfg.EmailFrom != "" || cfg.EmailFromName != "" {
			t.Errorf("メール設定 = %q / %q / %q、空を期待",
				cfg.ResendAPIKey, cfg.EmailFrom, cfg.EmailFromName)
		}
	})
}

// TestLoadReadsAppURLは任意のAppURLが公開HTTP・HTTPSベースURLを環境変数
// から受け入れ、未設定時は空になることを検証する (メール設定と同じ理由で必須ではない:
// 未設定でも起動できる必要がある)。
func TestLoadReadsAppURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "HTTPS", value: "https://groobb.example.dev"},
		{name: "ポート付きのHTTP", value: "http://localhost:8080"},
		{name: "未設定ならエラー無しで空になる", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_APP_URL", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load()が予期しないエラーを返した: %v", err)
			}

			if cfg.AppURL != tt.value {
				t.Errorf("AppURL = %q、期待値 = %q", cfg.AppURL, tt.value)
			}
		})
	}
}

// TestLoadRejectsInvalidAppURLは、指定されたAppURLがアプリケーションのパスの
// 意味を変えずに連結できる絶対公開HTTP(S) ベースURLであることを検証します。エラーは
// 不正な値を与えた入力元を名指しし、運用者が修正すべき入力を分かるようにします。
func TestLoadRejectsInvalidAppURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "相対", value: "groobb.example.dev"},
		{name: "対応していないスキーム", value: "ftp://groobb.example.dev"},
		{name: "ホストが無い", value: "https:///groobb"},
		{name: "ユーザー情報", value: "https://alice@groobb.example.dev"},
		{name: "クエリ", value: "https://groobb.example.dev?view=full"},
		{name: "空のクエリ", value: "https://groobb.example.dev?"},
		{name: "フラグメント", value: "https://groobb.example.dev#top"},
		{name: "空のフラグメント", value: "https://groobb.example.dev#"},
		{name: "末尾スラッシュ", value: "https://groobb.example.dev/"},
		{name: "パス", value: "https://groobb.example.dev/community"},
	}

	for _, tt := range tests {
		t.Run("環境変数/"+tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_APP_URL", tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load()がAppURL %q を拒否しなかった", tt.value)
			}
			if got := err.Error(); !strings.Contains(got, "the environment variable GROOBB_APP_URL") {
				t.Errorf("エラー = %q、環境変数の入力元を期待", got)
			}
		})
	}

	t.Run("設定ファイルの入力元", func(t *testing.T) {
		setRequiredEnv(t)
		writeConfigFile(t, "[app]\nurl = \"https://groobb.example.dev/\"\n")

		_, err := Load()
		if err == nil {
			t.Fatal("Load()が設定ファイルの不正なAppURLを拒否しなかった")
		}
		if got := err.Error(); !strings.Contains(got, `"app.url" in the configuration file`) {
			t.Errorf("エラー = %q、設定ファイルの入力元を期待", got)
		}
	})
}

// Cloudflareが公開しているダミーキー。テストのフィクスチャとしてのみ使い、
// devの実行時デフォルトには使わない。
const (
	turnstileTestSiteKey   = "1x00000000000000000000AA"
	turnstileTestSecretKey = "1x0000000000000000000000000000000AA"
)

// TestLoadReadsTurnstileSettingsは任意のTurnstileキーが環境変数から読み込まれ、
// 未設定時は空になることを検証する (実キーを設定する運用で有効化するため必須ではない)。
func TestLoadReadsTurnstileSettings(t *testing.T) {
	t.Run("設定あり", func(t *testing.T) {
		setRequiredEnv(t)
		// DISABLEを未設定にして、検証対象のキーが空に落とされないようにする。
		t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
		t.Setenv("GROOBB_TURNSTILE_SITE_KEY", turnstileTestSiteKey)
		t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", turnstileTestSecretKey)

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load()が予期しないエラーを返した: %v", err)
		}

		if cfg.TurnstileSiteKey != turnstileTestSiteKey {
			t.Errorf("TurnstileSiteKey = %q、期待値 = %q", cfg.TurnstileSiteKey, turnstileTestSiteKey)
		}
		if cfg.TurnstileSecretKey != turnstileTestSecretKey {
			t.Errorf("TurnstileSecretKey = %q、期待値 = %q", cfg.TurnstileSecretKey, turnstileTestSecretKey)
		}
	})

	t.Run("未設定ならエラー無しで空になる", func(t *testing.T) {
		setRequiredEnv(t)
		t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
		t.Setenv("GROOBB_TURNSTILE_SITE_KEY", "")
		t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Turnstileのキーが無いときにLoad()が失敗した: %v", err)
		}

		if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
			t.Errorf("Turnstileのキー = %q / %q、空を期待",
				cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
		}
	})
}

// TestLoadTurnstileDisableはfail-closedの無効化ロジックを検証する。
// GROOBB_TURNSTILE_DISABLEは非本番環境では両キーを空にするが、本番環境では無視され
// (キーを保持する)。
func TestLoadTurnstileDisable(t *testing.T) {
	tests := []struct {
		name            string
		env             string
		disable         string
		wantKeysCleared bool
	}{
		{name: "devではDISABLEがキーを空にする", env: "dev", disable: "true", wantKeysCleared: true},
		{name: "testではDISABLEがキーを空にする", env: "test", disable: "true", wantKeysCleared: true},
		{name: "本番ではDISABLEが無視される", env: "prod", disable: "true", wantKeysCleared: false},
		{name: "DISABLEが未設定ならキーを保つ", env: "test", disable: "", wantKeysCleared: false},
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
				t.Fatalf("Load()が予期しないエラーを返した: %v", err)
			}

			if tt.wantKeysCleared {
				if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
					t.Errorf("キーが空になっていない: site=%q secret=%q",
						cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
				}
				return
			}

			if cfg.TurnstileSiteKey != turnstileTestSiteKey {
				t.Errorf("TurnstileSiteKey = %q、期待値 = %q", cfg.TurnstileSiteKey, turnstileTestSiteKey)
			}
			if cfg.TurnstileSecretKey != turnstileTestSecretKey {
				t.Errorf("TurnstileSecretKey = %q、期待値 = %q", cfg.TurnstileSecretKey, turnstileTestSecretKey)
			}
		})
	}
}

// TestLoadTurnstilePartialKeyWarningは、本番でTurnstileのキーが片方だけ設定
// されているとき (黙ってバイパスされる設定ミス) にLoadが警告し、両方設定・両方空・
// 非本番のときは警告しないことを検証する。
func TestLoadTurnstilePartialKeyWarning(t *testing.T) {
	tests := []struct {
		name      string
		env       string
		siteKey   string
		secretKey string
		wantWarn  bool
	}{
		{name: "prodでサイトキーだけなら警告する", env: "prod", siteKey: turnstileTestSiteKey, secretKey: "", wantWarn: true},
		{name: "prodでシークレットキーだけなら警告する", env: "prod", siteKey: "", secretKey: turnstileTestSecretKey, wantWarn: true},
		{name: "prodで両方のキーがあれば警告しない", env: "prod", siteKey: turnstileTestSiteKey, secretKey: turnstileTestSecretKey, wantWarn: false},
		{name: "prodで両方のキーが空なら警告しない", env: "prod", siteKey: "", secretKey: "", wantWarn: false},
		{name: "devでサイトキーだけなら警告しない", env: "dev", siteKey: turnstileTestSiteKey, secretKey: "", wantWarn: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("APP_ENV", tt.env)
			t.Setenv("GROOBB_TURNSTILE_DISABLE", "")
			t.Setenv("GROOBB_TURNSTILE_SITE_KEY", tt.siteKey)
			t.Setenv("GROOBB_TURNSTILE_SECRET_KEY", tt.secretKey)

			// 片方キー警告を検証できるよう、テスト中だけデフォルトのslog出力を
			// 捕捉し、終了後に元へ戻す。
			var buf bytes.Buffer
			original := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
			defer slog.SetDefault(original)

			if _, err := Load(); err != nil {
				t.Fatalf("Load()が予期しないエラーを返した: %v", err)
			}

			warned := strings.Contains(buf.String(), "片方のみ設定")
			if warned != tt.wantWarn {
				t.Errorf("片方キーの警告の出力 = %v、期待値 = %v (ログ: %q)", warned, tt.wantWarn, buf.String())
			}
		})
	}
}

// TestLoadDefaultsEnvToDevはAPP_ENVが空のとき "dev" が既定値になることを検証します。
func TestLoadDefaultsEnvToDev(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("APP_ENV", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.Env != "dev" {
		t.Errorf("Env = %q、期待値 = %q", cfg.Env, "dev")
	}
}

// TestLoadMissingRequiredEnvは必須の環境変数が欠けているときLoadが失敗することを検証します。
func TestLoadMissingRequiredEnv(t *testing.T) {
	tests := []struct {
		name  string
		unset string
	}{
		{name: "GROOBB_PORTが無い", unset: "GROOBB_PORT"},
		{name: "GROOBB_DATABASE_PATHが無い", unset: "GROOBB_DATABASE_PATH"},
		{name: "GROOBB_CONTINUATION_TOKEN_KEYが無い", unset: "GROOBB_CONTINUATION_TOKEN_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(tt.unset, "")

			if _, err := Load(); err == nil {
				t.Errorf("%s が無いときにLoad()のエラーがnilだった", tt.unset)
			}
		})
	}
}

// TestLoadRejectsShortContinuationTokenKeyは、推測しやすい短い鍵が通常のアプリ起動
// 経路から署名処理へ到達できないことを検証します。
func TestLoadRejectsShortContinuationTokenKey(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "too-short")

	if _, err := Load(); err == nil {
		t.Fatal("GROOBB_CONTINUATION_TOKEN_KEYが32バイト未満なのにLoad()が失敗しなかった")
	}
}

// TestLoadRejectsAnInvalidPortは、サーバーが待ち受けられないポートが設定名を挙げて
// 起動を止めることを検証する。bindの失敗として現れたり、0の場合にカーネルが割り当てた
// ポートで待ち受けるインスタンスになったりしないようにするため。
func TestLoadRejectsAnInvalidPort(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "数値ではない", value: "http"},
		{name: "範囲外", value: "70000"},
		{name: "ゼロ", value: "0"},
		{name: "負の値", value: "-1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_PORT", tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load()がGROOBB_PORT = %q を拒否しなかった", tt.value)
			}
			if !strings.Contains(err.Error(), "GROOBB_PORT") {
				t.Errorf("エラーが値の入力元を挙げていない: %v", err)
			}
		})
	}
}

// TestLoadReadsTrustedProxiesは一覧の書き方を検証する。単一のアドレスはそれ自身を、
// CIDRブロックはそのネットワークを表すこと、項目の前後の空白は項目の一部ではないこと、
// prefix長より下位のビットを持つ項目が1つの形に収まることである。IPv4-mapped IPv6の
// ブロックは、実行時のピアと同じアドレスファミリーになるよう同等のIPv4表現にする。
// IPv6のzoneは取り除き、解決がピアからzoneを落としたうえで問い合わせるのと同じ範囲を
// 項目が覆うようにする。
func TestLoadReadsTrustedProxies(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{
			name:  "単一のアドレスはそれ自身を表す",
			value: "127.0.0.1",
			want:  []string{"127.0.0.1/32"},
		},
		{
			name:  "IPv6アドレスはそれ自身を表す",
			value: "::1",
			want:  []string{"::1/128"},
		},
		{
			name:  "複数の項目はカンマで区切り、前後に空白を置ける",
			value: "127.0.0.1, ::1 , 10.0.0.0/8",
			want:  []string{"127.0.0.1/32", "::1/128", "10.0.0.0/8"},
		},
		{
			name:  "ブロックは書かれたアドレスによらず1つの形になる",
			value: "10.1.2.3/8",
			want:  []string{"10.0.0.0/8"},
		},
		{
			name:  "IPv4-mapped IPv6アドレスのブロックは同等のIPv4になる",
			value: "::ffff:127.0.0.1/128",
			want:  []string{"127.0.0.1/32"},
		},
		{
			name:  "IPv4-mapped IPv6ネットワークは同等のIPv4になる",
			value: "::ffff:10.1.2.3/104",
			want:  []string{"10.0.0.0/8"},
		},
		{
			name:  "zone付きで書いたIPv6アドレスはアドレスだけを表す",
			value: "fe80::1%eth0",
			want:  []string{"fe80::1/128"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_TRUSTED_PROXIES", tt.value)

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load()が予期しないエラーを返した: %v", err)
			}

			got := make([]string, 0, len(cfg.TrustedProxies))
			for _, prefix := range cfg.TrustedProxies {
				got = append(got, prefix.String())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("TrustedProxies = %v、期待値 = %v", got, tt.want)
			}
		})
	}
}

// TestLoadWithoutTrustedProxiesは、設定を書かないことがエラーにならず、信頼する
// プロキシが1つも無い状態になることを検証する。直接公開されているインスタンスに必要なのが
// これで、その場合に転送ヘッダーは一切読まれない。
func TestLoadWithoutTrustedProxies(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v、空を期待", cfg.TrustedProxies)
	}
}

// TestLoadRejectsAnInvalidTrustedProxyは、アドレスではない項目が、その項目と入力元を
// 挙げて起動を止めることを検証する。代わりに取り除くと、そのプロキシの背後にいる訪問者が
// プロキシ自身として記録され、動いている設定に見えてしまう。
func TestLoadRejectsAnInvalidTrustedProxy(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{name: "アドレスではない", value: "proxy.example.dev"},
		{name: "範囲外のアドレス", value: "10.0.0.256"},
		{name: "範囲外のprefix長", value: "10.0.0.0/33"},
		{name: "IPv6も覆うIPv4-mappedのprefix", value: "::ffff:127.0.0.1/95"},
		{name: "正しい項目に混じった1つの不正な項目", value: "127.0.0.1,proxy.example.dev"},
		{name: "余分なカンマが残した空の項目", value: "127.0.0.1,"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv("GROOBB_TRUSTED_PROXIES", tt.value)

			_, err := Load()
			if err == nil {
				t.Fatalf("Load()がGROOBB_TRUSTED_PROXIES = %q を拒否しなかった", tt.value)
			}
			if !strings.Contains(err.Error(), "GROOBB_TRUSTED_PROXIES") {
				t.Errorf("エラーが値の入力元を挙げていない: %v", err)
			}
		})
	}
}

// TestEnvHelpersはIsDev / IsTest / IsProductionヘルパーを検証します。
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
				t.Errorf("IsDev() = %v、期待値 = %v", got, tt.isDev)
			}
			if got := cfg.IsTest(); got != tt.isTest {
				t.Errorf("IsTest() = %v、期待値 = %v", got, tt.isTest)
			}
			if got := cfg.IsProduction(); got != tt.isProduction {
				t.Errorf("IsProduction() = %v、期待値 = %v", got, tt.isProduction)
			}
		})
	}
}

// TestGetAssetVersionは、開発環境では空でない動的な値を返し、それ以外の
// 環境では起動時に固定した静的なAssetVersionを返すことを検証します。
func TestGetAssetVersion(t *testing.T) {
	t.Parallel()

	t.Run("devでは空でない動的な値を返す", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Env: "dev"}
		if got := cfg.GetAssetVersion(); got == "" {
			t.Error("devでGetAssetVersion()が空だった")
		}
	})

	t.Run("dev以外では静的なAssetVersionを返す", func(t *testing.T) {
		t.Parallel()

		cfg := &Config{Env: "prod", AssetVersion: "abc123"}
		if got := cfg.GetAssetVersion(); got != "abc123" {
			t.Errorf("GetAssetVersion() = %q、期待値 = %q", got, "abc123")
		}
	})
}

// TestBuildAssetVersionはアセットバージョンのフォールバック順序を検証します。
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
			name:     "ビルド時に埋め込んだ値を優先する",
			stamped:  "19ae8301290f4dc0e814bd0298d9e5c73cda684c",
			revision: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
			want:     "19ae8301290f4dc0e814bd0298d9e5c73cda684c",
		},
		{
			name:     "ビルドのリビジョンにフォールバックする",
			stamped:  "",
			revision: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
			want:     "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
		},
		{
			name:     "どちらも無ければdevにフォールバックする",
			stamped:  "",
			revision: "",
			want:     "dev",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := buildAssetVersion(tt.stamped, tt.revision); got != tt.want {
				t.Errorf("buildAssetVersion(%q, %q) = %q、期待値 = %q", tt.stamped, tt.revision, got, tt.want)
			}
		})
	}
}

// TestVCSRevisionFromSettingsは、通常VCS設定を持たないテストバイナリ自身の
// ビルド情報に依存せず、リビジョンの抽出を検証します。
func TestVCSRevisionFromSettings(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name: "完全なリビジョンをそのまま返す",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6"},
			},
			want: "5b40e741a55ead5001d61c10a4774a0ccaa3a2d6",
		},
		{
			name: "短いリビジョンを保つ",
			settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: "abc123"},
			},
			want: "abc123",
		},
		{
			name: "リビジョンの設定が無ければ空を返す",
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
				t.Errorf("vcsRevisionFromSettings(%v) = %q、期待値 = %q", tt.settings, got, tt.want)
			}
		})
	}
}

// TestLoadEmailProviderDefaultsToResendは、プロバイダー未設定のときResendの
// transportが維持されることを検証する。この設定が存在する前のデプロイが変わらないため。
func TestLoadEmailProviderDefaultsToResend(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.EmailProvider != EmailProviderResend {
		t.Errorf("EmailProvider = %q、期待値 = %q", cfg.EmailProvider, EmailProviderResend)
	}
}

// TestLoadRejectsUnknownEmailProviderは、対応していないプロバイダーが指定された
// とき、いずれかへ黙ってフォールバックせず起動を止めることを検証する。
func TestLoadRejectsUnknownEmailProvider(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("GROOBB_EMAIL_PROVIDER", "sendmail")

	if _, err := Load(); err == nil {
		t.Fatal("Load()が未知のGROOBB_EMAIL_PROVIDERを拒否しなかった")
	}
}

// setSMTPEnvは妥当で完全なSMTP設定一式を設定する。各ケースはそこから1箇所だけを
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

// TestLoadReadsSMTPSettingsは、SMTPプロバイダーが選択されたときにリレーの設定が
// 設定へ読み込まれることを検証する。
func TestLoadReadsSMTPSettings(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.EmailProvider != EmailProviderSMTP {
		t.Errorf("EmailProvider = %q、期待値 = %q", cfg.EmailProvider, EmailProviderSMTP)
	}
	if cfg.SMTPHost != "smtp.example.dev" {
		t.Errorf("SMTPHost = %q、期待値 = %q", cfg.SMTPHost, "smtp.example.dev")
	}
	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d、期待値 = 587", cfg.SMTPPort)
	}
	if cfg.SMTPUsername != "smtp-user" {
		t.Errorf("SMTPUsername = %q、期待値 = %q", cfg.SMTPUsername, "smtp-user")
	}
	if cfg.SMTPPassword != "smtp-password" {
		t.Errorf("SMTPPasswordが環境変数から読み込まれていない")
	}
	if cfg.SMTPTLSMode != smtpTLSModeStartTLS {
		t.Errorf("SMTPTLSMode = %q、期待値 = %q", cfg.SMTPTLSMode, smtpTLSModeStartTLS)
	}
}

// TestLoadDefaultsSMTPTLSModeToStartTLSは、TLSモード未設定のときに接続を平文の
// ままにせず保護することを検証する。
func TestLoadDefaultsSMTPTLSModeToStartTLS(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)
	t.Setenv("GROOBB_SMTP_TLS_MODE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.SMTPTLSMode != smtpTLSModeStartTLS {
		t.Errorf("SMTPTLSMode = %q、期待値 = %q", cfg.SMTPTLSMode, smtpTLSModeStartTLS)
	}
}

// TestLoadAcceptsSMTPTLSModesは、サポートするすべてのモードを環境変数との
// 境界で固定する。Senderの直接テストが通り続ける一方で、起動設定からいずれかの
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
				t.Fatalf("Load()が予期しないエラーを返した: %v", err)
			}

			if cfg.SMTPTLSMode != mode {
				t.Errorf("SMTPTLSMode = %q、期待値 = %q", cfg.SMTPTLSMode, mode)
			}
		})
	}
}

// TestLoadAcceptsSMTPWithoutCredentialsは、送信元アドレスで認可するリレーが妥当な
// 設定であることを検証する。
func TestLoadAcceptsSMTPWithoutCredentials(t *testing.T) {
	setRequiredEnv(t)
	setSMTPEnv(t)
	t.Setenv("GROOBB_SMTP_USERNAME", "")
	t.Setenv("GROOBB_SMTP_PASSWORD", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.SMTPUsername != "" || cfg.SMTPPassword != "" {
		t.Error("どちらの変数も設定していないのに認証情報が空ではない")
	}
}

// TestLoadRejectsIncompleteSMTPSettingsはリレー設定が誤りうる各ケースを確認する。
// 運用者が、届かないメールではなく起動時エラーを受け取るようにするため。
func TestLoadRejectsIncompleteSMTPSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "ホストが無い", key: "GROOBB_SMTP_HOST", value: ""},
		{name: "ポートが無い", key: "GROOBB_SMTP_PORT", value: ""},
		{name: "ポートが数値ではない", key: "GROOBB_SMTP_PORT", value: "submission"},
		{name: "ポートが範囲外", key: "GROOBB_SMTP_PORT", value: "70000"},
		{name: "ポートがゼロ", key: "GROOBB_SMTP_PORT", value: "0"},
		{name: "パスワードの無いユーザー名", key: "GROOBB_SMTP_PASSWORD", value: ""},
		{name: "ユーザー名の無いパスワード", key: "GROOBB_SMTP_USERNAME", value: ""},
		{name: "未知のTLSモード", key: "GROOBB_SMTP_TLS_MODE", value: "ssl"},
		{name: "送信元アドレスが無い", key: "GROOBB_EMAIL_FROM", value: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequiredEnv(t)
			setSMTPEnv(t)
			t.Setenv(tt.key, tt.value)

			if _, err := Load(); err == nil {
				t.Fatalf("Load()が %s = %q を拒否しなかった", tt.key, tt.value)
			}
		})
	}
}

// TestLoadWarnsOnUnencryptedSMTPInProductionは、暗号化しないリレーが許容されつつ
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
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if !strings.Contains(buf.String(), "SMTP") {
		t.Errorf("暗号化しないリレーについての警告を期待したが、出力は: %s", buf.String())
	}
}

// TestSMTPTLSModeValuesMatchEmailPackageは、本パッケージが再掲しているリテラルを
// emailパッケージの定数に固定する。実行時に両者を繋ぐのはワーカーの検査を伴わない文字列
// 変換だけなので、片側の値がずれてもここでは読み込めてしまい、Sender側ではSTARTTLSの
// 既定へ落ちてしまう。
//
// このimportはテスト専用であり、本番のconfigパッケージがemailパッケージをimport
// しないようにする。emailパッケージはメールテンプレートを引き込む一方、configはほぼ
// すべてのパッケージからimportされるためである。
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
				t.Errorf("configの値 = %q、期待値 = emailパッケージの %q", tt.got, tt.want)
			}
		})
	}
}

// TestLogValueRedactsSecretsは、Configをログに出してもインスタンスの認証に使う値が
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
		{name: "ポインター", value: &cfg},
		{name: "値", value: cfg},
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
					t.Errorf("ログに秘密情報 %q が含まれている: %s", secret, logged)
				}
			}

			if !strings.Contains(logged, redactedSecret) {
				t.Errorf("ログが設定済みの秘密情報を示していない: %s", logged)
			}
			if !strings.Contains(logged, cfg.SMTPHost) {
				t.Errorf("ログに秘密情報でない設定が残っていない: %s", logged)
			}
		})
	}
}
