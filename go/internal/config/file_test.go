package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fullConfigFileはすべての設定を書いたファイルです。ケースが名指しした設定だけで
// なく、ファイル全体が読まれることを検証できるようにするためです。
const fullConfigFile = `
[app]
env = "prod"
url = "https://groobb.example.dev"

[server]
port = 9090
trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]

[database]
path = "/var/lib/groobb/groobb.sqlite"

[security]
continuation_token_key = "groobb-test-continuation-token-key-32-bytes"

[email]
provider = "smtp"
from = "noreply@example.dev"
from_name = "Groobb"
resend_api_key = "re_test_key"

[email.smtp]
host = "smtp.example.dev"
port = 587
username = "smtp-user"
password = "smtp-password"
tls_mode = "implicit"

[turnstile]
site_key = "1x00000000000000000000AA"
secret_key = "1x0000000000000000000000000000000AA"
disable = false
`

// writeConfigFileは、clearEnvがテストを移した作業ディレクトリの中に、既定の
// パスで設定ファイルを書き出します。
func writeConfigFile(t *testing.T, contents string) {
	t.Helper()

	if err := os.WriteFile(defaultConfigFileName, []byte(contents), 0o600); err != nil {
		t.Fatalf("設定ファイルの書き込みに失敗: %v", err)
	}
}

// TestLoadReadsSettingsFromConfigFileは、ファイルだけでインスタンスを設定できる
// ことを検証する。セルフホストのデプロイが頼りにするのはこの形であるため。
func TestLoadReadsSettingsFromConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, fullConfigFile)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{name: "Env", got: cfg.Env, want: "prod"},
		{name: "AppURL", got: cfg.AppURL, want: "https://groobb.example.dev"},
		{name: "Port", got: cfg.Port, want: "9090"},
		{name: "DatabasePath", got: cfg.DatabasePath, want: "/var/lib/groobb/groobb.sqlite"},
		{name: "ContinuationTokenKey", got: cfg.ContinuationTokenKey, want: "groobb-test-continuation-token-key-32-bytes"},
		{name: "EmailProvider", got: cfg.EmailProvider, want: EmailProviderSMTP},
		{name: "EmailFrom", got: cfg.EmailFrom, want: "noreply@example.dev"},
		{name: "EmailFromName", got: cfg.EmailFromName, want: "Groobb"},
		{name: "ResendAPIKey", got: cfg.ResendAPIKey, want: "re_test_key"},
		{name: "SMTPHost", got: cfg.SMTPHost, want: "smtp.example.dev"},
		{name: "SMTPUsername", got: cfg.SMTPUsername, want: "smtp-user"},
		{name: "SMTPPassword", got: cfg.SMTPPassword, want: "smtp-password"},
		{name: "SMTPTLSMode", got: cfg.SMTPTLSMode, want: smtpTLSModeImplicit},
		{name: "TurnstileSiteKey", got: cfg.TurnstileSiteKey, want: turnstileTestSiteKey},
		{name: "TurnstileSecretKey", got: cfg.TurnstileSecretKey, want: turnstileTestSecretKey},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q、期待値 = %q", tt.name, tt.got, tt.want)
		}
	}

	if cfg.SMTPPort != 587 {
		t.Errorf("SMTPPort = %d、期待値 = 587", cfg.SMTPPort)
	}

	// 信頼するプロキシはスカラーではなくリストのため、上の表とは別に検証する。
	// 整形した形は、解決がアドレスを照合する相手そのものである。
	wantTrustedProxies := []string{"127.0.0.1/32", "10.0.0.0/8"}
	gotTrustedProxies := make([]string, 0, len(cfg.TrustedProxies))
	for _, prefix := range cfg.TrustedProxies {
		gotTrustedProxies = append(gotTrustedProxies, prefix.String())
	}
	if !slices.Equal(gotTrustedProxies, wantTrustedProxies) {
		t.Errorf("TrustedProxies = %v、期待値 = %v", gotTrustedProxies, wantTrustedProxies)
	}
}

// TestLoadRejectsAnInvalidTrustedProxyFromTheConfigFileは、ファイルの不正な項目が
// 環境変数のものと同じように報告され、変数ではなくファイルのキーを挙げることを検証する。
// 運用者が自分の書いた場所を見に行けるようにするためである。
func TestLoadRejectsAnInvalidTrustedProxyFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, `trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]`, `trusted_proxies = ["proxy.example.dev"]`, 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load()がアドレスではない信頼するプロキシを拒否しなかった")
	}
	if !strings.Contains(err.Error(), "server.trusted_proxies") {
		t.Errorf("エラーが値の由来するファイルのキーを挙げていない: %v", err)
	}
}

// TestLoadRejectsAnEmptyTrustedProxyFromTheConfigFileは、空文字列だけの配列が、
// 誰も書いていないキーとして読まれるのではなく報告されることを検証する。空のまま残った項目は、
// それが唯一の項目でも複数のうちの1つでも同じ誤りなので、どちらでも運用者に伝わるように
// する。
func TestLoadRejectsAnEmptyTrustedProxyFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, `trusted_proxies = ["127.0.0.1", "10.0.0.0/8"]`, `trusted_proxies = [""]`, 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load()が空の項目を含む信頼するプロキシの一覧を拒否しなかった")
	}
	if !strings.Contains(err.Error(), "server.trusted_proxies") {
		t.Errorf("エラーが値の由来するファイルのキーを挙げていない: %v", err)
	}
}

// TestLoadPrefersTheEnvironmentOverTheConfigFileは、両方に書かれた設定が環境変数の
// 値になり、環境変数が触れていない設定はファイルの値を保つことを検証する。
func TestLoadPrefersTheEnvironmentOverTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, fullConfigFile)
	t.Setenv("GROOBB_PORT", "8080")
	t.Setenv("GROOBB_SMTP_PASSWORD", "smtp-password-from-the-environment")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q、期待値 = 環境変数の値 %q", cfg.Port, "8080")
	}
	if cfg.SMTPPassword != "smtp-password-from-the-environment" {
		t.Error("環境変数に設定したSMTPPasswordが環境変数の値になっていない")
	}
	if cfg.SMTPUsername != "smtp-user" {
		t.Errorf("SMTPUsername = %q、期待値 = ファイルの値 %q", cfg.SMTPUsername, "smtp-user")
	}
}

// TestLoadWithoutAConfigFileは、既定のファイルが無いことがエラーにならず、環境変数
// だけで設定したインスタンスが起動することを検証する。
func TestLoadWithoutAConfigFile(t *testing.T) {
	setRequiredEnv(t)

	if _, err := os.Stat(defaultConfigFileName); !os.IsNotExist(err) {
		t.Fatalf("作業ディレクトリに設定ファイルが無いことを期待したが、Stat()の結果 = %v", err)
	}

	if _, err := Load(); err != nil {
		t.Fatalf("設定ファイルが無いときにLoad()が失敗した: %v", err)
	}
}

// TestLoadReadsTheConfigFileNamedByTheEnvironmentは、運用者が指定したときに作業
// ディレクトリの外にあるファイルが読まれることを検証する。
func TestLoadReadsTheConfigFileNamedByTheEnvironment(t *testing.T) {
	clearEnv(t)

	path := filepath.Join(t.TempDir(), "instance.toml")
	if err := os.WriteFile(path, []byte(fullConfigFile), 0o600); err != nil {
		t.Fatalf("設定ファイルの書き込みに失敗: %v", err)
	}
	t.Setenv(configFileEnvName, path)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("Port = %q、期待値 = %q (%s が指すファイルの値)", cfg.Port, "9090", configFileEnvName)
	}
}

// TestLoadRejectsAMissingExplicitConfigFileは、運用者が指定したパスが黙って無視され
// ないことを検証する。そうでなければ、パスの打ち間違いがファイルの設定をすべて捨てた
// インスタンスを起動させてしまう。
func TestLoadRejectsAMissingExplicitConfigFile(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv(configFileEnvName, filepath.Join(t.TempDir(), "absent.toml"))

	if _, err := Load(); err == nil {
		t.Fatalf("%s が指すファイルが無いのにLoad()が失敗しなかった", configFileEnvName)
	}
}

// TestLoadRejectsUnknownConfigFileKeysは、綴りを誤った設定が起動を止め、その名前が
// 示されることを検証する。設定を書いていないのと同じ状態で動くインスタンスを残さないため。
func TestLoadRejectsUnknownConfigFileKeys(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[email.smtp]\nhostname = \"smtp.example.dev\"\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load()がどの設定にも一致しないキーを拒否しなかった")
	}
	if !strings.Contains(err.Error(), "email.smtp.hostname") {
		t.Errorf("エラーが未知のキーを挙げていない: %v", err)
	}
}

// TestLoadConfigFileSyntaxErrorOmitsTheFileContentsは、構文エラーが位置で報告され、
// ファイルの記述内容を引用しないことを検証する。パーサー自身のメッセージはつまずいた
// トークンを引用し、そのトークンはパスワードでありうる。
func TestLoadConfigFileSyntaxErrorOmitsTheFileContents(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[email.smtp]\npassword = hunter2\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load()が解析できない設定ファイルを拒否しなかった")
	}
	if strings.Contains(err.Error(), "hunter") {
		t.Errorf("エラーがファイルの記述内容を引用している: %v", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Errorf("エラーが問題の位置を挙げていない: %v", err)
	}
}

// TestLoadMissingSettingNamesBothSourcesは、どちらの入力にも無い必須の設定が両方の
// 名前とともに報告されることを検証する。どちらで設定している運用者もメッセージから対処
// できるようにするため。
func TestLoadMissingSettingNamesBothSources(t *testing.T) {
	clearEnv(t)

	_, err := Load()
	if err == nil {
		t.Fatal("どの入力元もポートを設定していないのにLoad()が失敗しなかった")
	}
	if !strings.Contains(err.Error(), "GROOBB_PORT") || !strings.Contains(err.Error(), "server.port") {
		t.Errorf("エラーが設定の両方の入力元を挙げていない: %v", err)
	}
}

// TestLoadInvalidSettingNamesTheSourceItCameFromは、ファイル由来の不正な値がファイル
// に対して報告されることを検証する。代わりに環境変数を名指しすると、運用者は何も設定して
// いない場所を探すことになる。
func TestLoadInvalidSettingNamesTheSourceItCameFrom(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(fullConfigFile, "port = 587", "port = 70000", 1))

	_, err := Load()
	if err == nil {
		t.Fatal("Load()が有効範囲外のSMTPポートを拒否しなかった")
	}
	if !strings.Contains(err.Error(), "email.smtp.port") {
		t.Errorf("エラーが値の由来するファイルのキーを挙げていない: %v", err)
	}
	if strings.Contains(err.Error(), "GROOBB_SMTP_PORT") {
		t.Errorf("エラーが、ここでは何も設定していない環境変数を挙げている: %v", err)
	}
}

// TestLoadReadsTheTurnstileDisableFlagFromTheConfigFileは、真偽値の設定がファイル
// から機能することを検証する。真偽値として書かれるのはファイルだけであるため。
func TestLoadReadsTheTurnstileDisableFlagFromTheConfigFile(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, strings.Replace(
		strings.Replace(fullConfigFile, `env = "prod"`, `env = "test"`, 1),
		"disable = false", "disable = true", 1))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load()が予期しないエラーを返した: %v", err)
	}

	if cfg.TurnstileSiteKey != "" || cfg.TurnstileSecretKey != "" {
		t.Errorf("キーが空になっていない: site=%q secret=%q", cfg.TurnstileSiteKey, cfg.TurnstileSecretKey)
	}
}

// TestExampleConfigFileLoadsは、サーバーに同梱するサンプルファイルが実際に動く設定で
// あることを検証する。セルフホストの設定はこのファイルのコピーから始まり、デコードは厳密で
// あるため、スキーマからずれたキーはコピーした人のインスタンスを止めてしまう。
func TestExampleConfigFileLoads(t *testing.T) {
	// パスは、clearEnvがテストをパッケージのディレクトリから移す前に解決する。
	path, err := filepath.Abs(filepath.Join("..", "..", "groobb.example.toml"))
	if err != nil {
		t.Fatalf("サンプル設定ファイルのパスの解決に失敗: %v", err)
	}

	clearEnv(t)
	// サンプルは、コピーしたファイルが公開済みの鍵やプレースホルダーのリレーで起動
	// しないよう、continuation tokenの鍵とSMTPのホストを意図的に空にしている。残りの
	// サンプルをLoadで確認する間だけ、両方にテスト用の値を与える。
	t.Setenv("GROOBB_CONTINUATION_TOKEN_KEY", "groobb-test-continuation-token-key-32-bytes")
	t.Setenv("GROOBB_SMTP_HOST", "smtp.example.dev")
	t.Setenv(configFileEnvName, path)

	if _, err := Load(); err != nil {
		t.Fatalf("サンプル設定ファイルの読み込みに失敗: %v", err)
	}
}

// TestLoadConfigFileTypeErrorOmitsTheValueは、パーサーが型として報告するデコード
// エラーが値を伴わずに通され、かつファイル名を挙げることを検証する。configFileErrorが
// この種のエラーのメッセージを残しているのは、それが値ではなく型を報告するからであり、
// 本テストはその前提を固定する。値を引用するようになったライブラリは、秘密情報を起動時の
// ログへ載せることになる。
func TestLoadConfigFileTypeErrorOmitsTheValue(t *testing.T) {
	clearEnv(t)
	writeConfigFile(t, "[security]\ncontinuation_token_key = 1234567890\n")

	_, err := Load()
	if err == nil {
		t.Fatal("Load()が設定の型に合わない値を拒否しなかった")
	}
	if strings.Contains(err.Error(), "1234567890") {
		t.Errorf("エラーがファイルの記述内容を引用している: %v", err)
	}
	if !strings.Contains(err.Error(), defaultConfigFileName) {
		t.Errorf("エラーが失敗したファイルを挙げていない: %v", err)
	}
}

// TestLoadRejectsPortZeroFromTheConfigFileは、0と書かれたポートが範囲外の値として
// 報告されることを検証する。ファイルは数値を数値として運ぶため0はフィールドのゼロ値に
// あたるが、これを「誰も設定していない」と報告すると、書いた運用者に書いていないと伝える
// ことになる。ポート0はカーネルには「空いている任意のポート」として届く。
func TestLoadRejectsPortZeroFromTheConfigFile(t *testing.T) {
	tests := []struct {
		name    string
		old     string
		fileKey string
	}{
		{name: "server", old: "port = 9090", fileKey: "server.port"},
		{name: "SMTP", old: "port = 587", fileKey: "email.smtp.port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			writeConfigFile(t, strings.Replace(fullConfigFile, tt.old, "port = 0", 1))

			_, err := Load()
			if err == nil {
				t.Fatalf("Load()が %s の値0を拒否しなかった", tt.fileKey)
			}
			if !strings.Contains(err.Error(), "between 1 and 65535") {
				t.Errorf("エラーが値を範囲外として報告していない: %v", err)
			}
			if !strings.Contains(err.Error(), tt.fileKey) {
				t.Errorf("エラーが値の由来するファイルのキーを挙げていない: %v", err)
			}
		})
	}
}
