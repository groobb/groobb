package email

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/wneessen/go-mail"
)

// Compile-time assertion that the SMTP sender satisfies Sender.
//
// [Ja] SMTP の Sender が Sender を満たすことのコンパイル時表明。
var _ Sender = (*SMTPSender)(nil)

// newTestSMTPSender points a sender at the fake server and trusts its
// certificate, so the TLS modes can be exercised against a self-signed
// certificate.
//
// [Ja] newTestSMTPSender は Sender をテスト用サーバーへ向け、その証明書を信頼させる。
// これにより自己署名証明書に対して各 TLS モードを動かせる。
func newTestSMTPSender(t *testing.T, server *fakeSMTPServer, mode SMTPTLSMode, username, password string) *SMTPSender {
	t.Helper()

	host, port := server.hostPort(t)
	sender := NewSMTPSender(SMTPConfig{
		Host:      host,
		Port:      port,
		Username:  username,
		Password:  password,
		TLSMode:   mode,
		FromEmail: "noreply@example.dev",
		FromName:  "Groobb",
	})
	sender.tlsConfig = server.clientTLSConfig(host)

	return sender
}

func testSendInput() SendInput {
	return SendInput{
		To:       "user@example.dev",
		Subject:  "確認用コード",
		HTMLBody: templ.Raw("<p>HTML_MARKER</p>"),
		TextBody: templ.Raw("TEXT_MARKER"),
	}
}

// TestSMTPSender_Send checks the delivered message over a plaintext connection:
// the envelope recipient, the From header, and a multipart/alternative body whose
// HTML part comes last so clients prefer it.
//
// [Ja] TestSMTPSender_Send は平文接続で配送されたメッセージを確認する。エンベロープの
// 宛先、From ヘッダー、そして HTML パートが後ろに来る multipart/alternative の本文
// (クライアントが HTML を優先する条件) を検証する。
func TestSMTPSender_Send(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	messages := server.received()
	if len(messages) != 1 {
		t.Fatalf("len(received()) = %d, want 1", len(messages))
	}
	message := messages[0]

	if got := server.rcptTo(); len(got) != 1 || !strings.Contains(got[0], "user@example.dev") {
		t.Errorf("rcptTo() = %v, want one entry containing %q", got, "user@example.dev")
	}
	if !strings.Contains(message, `From: "Groobb" <noreply@example.dev>`) {
		t.Errorf("From header is missing from the message:\n%s", message)
	}
	if !strings.Contains(message, "multipart/alternative") {
		t.Errorf("Content-Type is not multipart/alternative:\n%s", message)
	}

	textIndex := strings.Index(message, "text/plain")
	htmlIndex := strings.Index(message, "text/html")
	if textIndex < 0 || htmlIndex < 0 {
		t.Fatalf("both body parts should be present:\n%s", message)
	}
	if textIndex > htmlIndex {
		t.Error("the text part should come before the HTML part so clients prefer the HTML one")
	}
}

// TestSMTPSender_Send_EncodesNonASCIISubject verifies the subject travels as an
// RFC 2047 encoded word rather than as raw UTF-8, which a relay may reject or
// mangle.
//
// [Ja] TestSMTPSender_Send_EncodesNonASCIISubject は件名が生の UTF-8 ではなく
// RFC 2047 の encoded word として送られることを検証する。生の UTF-8 はリレーに
// 拒否されたり壊されたりしうる。
func TestSMTPSender_Send_EncodesNonASCIISubject(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "確認用コード") {
		t.Errorf("the subject was sent unencoded:\n%s", message)
	}
	if !strings.Contains(message, "=?UTF-8?") {
		t.Errorf("the subject is not an RFC 2047 encoded word:\n%s", message)
	}
}

// TestSMTPSender_Send_HeaderInjection confirms a line break in the subject cannot
// introduce a header of the attacker's choosing.
//
// [Ja] TestSMTPSender_Send_HeaderInjection は件名の改行によって攻撃者の望むヘッダーを
// 差し込めないことを確認する。
func TestSMTPSender_Send_HeaderInjection(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	input := testSendInput()
	input.Subject = "Subject\r\nBcc: attacker@example.dev"
	if err := sender.Send(context.Background(), input); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "\r\nBcc:") {
		t.Errorf("a Bcc header was injected through the subject:\n%s", message)
	}
	if got := server.rcptTo(); len(got) != 1 {
		t.Errorf("rcptTo() = %v, want a single recipient", got)
	}
}

// TestSMTPSender_Send_HTMLOnly covers a SendInput without a text body: the HTML
// becomes the whole body instead of one part of an alternative.
//
// [Ja] TestSMTPSender_Send_HTMLOnly はテキスト本文を持たない SendInput を扱う。この
// 場合 HTML は alternative の 1 パートではなく本文そのものになる。
func TestSMTPSender_Send_HTMLOnly(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	input := testSendInput()
	input.TextBody = nil
	if err := sender.Send(context.Background(), input); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "multipart/alternative") {
		t.Errorf("a message without a text body should not be multipart:\n%s", message)
	}
	if !strings.Contains(message, "text/html") {
		t.Errorf("the HTML body is missing:\n%s", message)
	}
}

// TestSMTPSender_Send_Authenticates verifies credentials are offered when a
// username is configured, and that no AUTH is attempted when it is empty. It
// runs over STARTTLS because the client refuses to offer a password over an
// unencrypted connection.
//
// [Ja] TestSMTPSender_Send_Authenticates は、ユーザー名が設定されているときに認証情報を
// 提示し、空のときは AUTH を試みないことを検証する。クライアントは暗号化されていない接続で
// パスワードを提示しないため、STARTTLS 上で実行する。
func TestSMTPSender_Send_Authenticates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		username     string
		password     string
		wantAttempts int
	}{
		{name: "with credentials", username: "smtp-user", password: "smtp-password", wantAttempts: 1},
		{name: "without credentials", username: "", password: "", wantAttempts: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTPServer(t, false, true)
			sender := newTestSMTPSender(t, server, SMTPTLSModeStartTLS, tt.username, tt.password)

			if err := sender.Send(context.Background(), testSendInput()); err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if got := server.authenticated(); len(got) != tt.wantAttempts {
				t.Errorf("authenticated() = %v, want %d attempt(s)", got, tt.wantAttempts)
			}
		})
	}
}

// TestSMTPSender_Send_UnencryptedWithCredentials pins what a password does on the
// unencrypted mode: the client offers no mechanism the relay advertises, so the
// send fails and nothing is delivered. Fixing this keeps a library upgrade from
// silently turning the combination into PLAIN over a connection in the clear.
//
// [Ja] TestSMTPSender_Send_UnencryptedWithCredentials は、暗号化しないモードで
// パスワードを設定したときの挙動を固定する。クライアントはリレーが広告する方式を
// 1 つも提示できないため送信が失敗し、何も配送されない。これを固定しておくことで、
// ライブラリの更新によってこの組み合わせが平文接続上の PLAIN へ黙って変わることを防ぐ。
func TestSMTPSender_Send_UnencryptedWithCredentials(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "smtp-user", "smtp-password")

	if err := sender.Send(context.Background(), testSendInput()); err == nil {
		t.Fatal("Send() should fail rather than offer the password over an unencrypted connection")
	}

	if got := server.authenticated(); len(got) != 0 {
		t.Errorf("authenticated() = %v, want no attempt", got)
	}
	if len(server.received()) != 0 {
		t.Error("nothing should be delivered when no mechanism can be used")
	}
}

// TestSMTPSender_Send_TLSModes exercises delivery over each secured mode against
// a server speaking that mode.
//
// [Ja] TestSMTPSender_Send_TLSModes は保護された各モードでの配送を、そのモードを話す
// サーバーに対して動かす。
func TestSMTPSender_Send_TLSModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		mode          SMTPTLSMode
		implicitTLS   bool
		offerStartTLS bool
	}{
		{name: "starttls", mode: SMTPTLSModeStartTLS, implicitTLS: false, offerStartTLS: true},
		{name: "implicit", mode: SMTPTLSModeImplicit, implicitTLS: true, offerStartTLS: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTPServer(t, tt.implicitTLS, tt.offerStartTLS)
			sender := newTestSMTPSender(t, server, tt.mode, "smtp-user", "smtp-password")

			if err := sender.Send(context.Background(), testSendInput()); err != nil {
				t.Fatalf("Send() error = %v", err)
			}

			if len(server.received()) != 1 {
				t.Fatalf("len(received()) = %d, want 1", len(server.received()))
			}
			if got := server.authenticated(); len(got) != 1 {
				t.Errorf("authenticated() = %v, want 1 attempt", got)
			}
		})
	}
}

// TestSMTPSender_Send_StartTLSRequired verifies the STARTTLS mode refuses to
// deliver over a relay that does not offer STARTTLS, rather than silently falling
// back to an unencrypted connection.
//
// [Ja] TestSMTPSender_Send_StartTLSRequired は、STARTTLS モードが STARTTLS を提供
// しないリレーに対して、暗号化されない接続へ黙ってフォールバックせず配送を拒否することを
// 検証する。
func TestSMTPSender_Send_StartTLSRequired(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeStartTLS, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err == nil {
		t.Fatal("Send() should fail when the relay does not offer STARTTLS")
	}
	if len(server.received()) != 0 {
		t.Error("nothing should be delivered when STARTTLS is unavailable")
	}
}

// TestSMTPSender_clientOptions_TLSPolicy pins the policy each mode produces,
// including the fallback for a mode outside the three constants.
//
// [Ja] TestSMTPSender_clientOptions_TLSPolicy は各モードが生成するポリシーを固定する。
// 3 つの定数以外のモードのフォールバックも含む。
func TestSMTPSender_clientOptions_TLSPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode SMTPTLSMode
		want string
	}{
		{name: "starttls", mode: SMTPTLSModeStartTLS, want: mail.TLSMandatory.String()},
		{name: "none", mode: SMTPTLSModeNone, want: mail.NoTLS.String()},
		{name: "unknown falls back to mandatory", mode: SMTPTLSMode("bogus"), want: mail.TLSMandatory.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sender := NewSMTPSender(SMTPConfig{Host: "smtp.example.dev", Port: 587, TLSMode: tt.mode})
			client, err := mail.NewClient(sender.config.Host, sender.clientOptions()...)
			if err != nil {
				t.Fatalf("mail.NewClient() error = %v", err)
			}
			if got := client.TLSPolicy(); got != tt.want {
				t.Errorf("TLSPolicy() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewSMTPSender_TLSConfig checks the sender pins a minimum TLS version and
// verifies the relay's certificate against its configured host name.
//
// [Ja] TestNewSMTPSender_TLSConfig は Sender が最小 TLS バージョンを固定し、設定された
// ホスト名でリレーの証明書を検証することを確認する。
func TestNewSMTPSender_TLSConfig(t *testing.T) {
	t.Parallel()

	sender := NewSMTPSender(SMTPConfig{Host: "smtp.example.dev", Port: 587})

	if sender.tlsConfig.ServerName != "smtp.example.dev" {
		t.Errorf("ServerName = %q, want %q", sender.tlsConfig.ServerName, "smtp.example.dev")
	}
	if sender.tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d, want %d", sender.tlsConfig.MinVersion, tls.VersionTLS12)
	}
	if sender.tlsConfig.InsecureSkipVerify {
		t.Error("certificate verification should stay on")
	}
}
