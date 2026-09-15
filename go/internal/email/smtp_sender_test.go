package email

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/wneessen/go-mail"
)

// SMTPのSenderがSenderを満たすことのコンパイル時表明。
var _ Sender = (*SMTPSender)(nil)

// newTestSMTPSenderはSenderをテスト用サーバーへ向け、その証明書を信頼させる。
// これにより自己署名証明書に対して各TLSモードを動かせる。
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

// TestSMTPSender_Sendは平文接続で配送されたメッセージを確認する。エンベロープの
// 宛先、Fromヘッダー、そしてHTMLパートが後ろに来るmultipart/alternativeの本文
// (クライアントがHTMLを優先する条件) を検証する。
func TestSMTPSender_Send(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	messages := server.received()
	if len(messages) != 1 {
		t.Fatalf("len(received()) = %d、期待値 = 1", len(messages))
	}
	message := messages[0]

	if got := server.rcptTo(); len(got) != 1 || !strings.Contains(got[0], "user@example.dev") {
		t.Errorf("rcptTo() = %v、期待値は %q を含む1件", got, "user@example.dev")
	}
	if !strings.Contains(message, `From: "Groobb" <noreply@example.dev>`) {
		t.Errorf("メッセージにFromヘッダーが無い:\n%s", message)
	}
	if !strings.Contains(message, "multipart/alternative") {
		t.Errorf("Content-Typeがmultipart/alternativeでない:\n%s", message)
	}

	textIndex := strings.Index(message, "text/plain")
	htmlIndex := strings.Index(message, "text/html")
	if textIndex < 0 || htmlIndex < 0 {
		t.Fatalf("本文の両パートがそろっていない:\n%s", message)
	}
	if textIndex > htmlIndex {
		t.Error("テキストパートがHTMLパートより前に来ていない (クライアントにHTMLを優先させるには前に来るべき)")
	}
}

// TestSMTPSender_Send_EncodesNonASCIISubjectは件名が生のUTF-8ではなく
// RFC 2047のencoded wordとして送られることを検証する。生のUTF-8はリレーに
// 拒否されたり壊されたりしうる。
func TestSMTPSender_Send_EncodesNonASCIISubject(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "確認用コード") {
		t.Errorf("件名がエンコードされずに送られた:\n%s", message)
	}
	if !strings.Contains(message, "=?UTF-8?") {
		t.Errorf("件名がRFC 2047のencoded wordでない:\n%s", message)
	}
}

// TestSMTPSender_Send_HeaderInjectionは件名の改行によって攻撃者の望むヘッダーを
// 差し込めないことを確認する。
func TestSMTPSender_Send_HeaderInjection(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	input := testSendInput()
	input.Subject = "Subject\r\nBcc: attacker@example.dev"
	if err := sender.Send(context.Background(), input); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "\r\nBcc:") {
		t.Errorf("件名を通じてBccヘッダーが差し込まれた:\n%s", message)
	}
	if got := server.rcptTo(); len(got) != 1 {
		t.Errorf("rcptTo() = %v、期待値は宛先1件", got)
	}
}

// TestSMTPSender_Send_HTMLOnlyはテキスト本文を持たないSendInputを扱う。この
// 場合HTMLはalternativeの1パートではなく本文そのものになる。
func TestSMTPSender_Send_HTMLOnly(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "", "")

	input := testSendInput()
	input.TextBody = nil
	if err := sender.Send(context.Background(), input); err != nil {
		t.Fatalf("Send()のエラー = %v", err)
	}

	message := server.received()[0]
	if strings.Contains(message, "multipart/alternative") {
		t.Errorf("テキスト本文を持たないメッセージがmultipartになっている:\n%s", message)
	}
	if !strings.Contains(message, "text/html") {
		t.Errorf("HTML本文が無い:\n%s", message)
	}
}

// TestSMTPSender_Send_Authenticatesは、ユーザー名が設定されているときに認証情報を
// 提示し、空のときはAUTHを試みないことを検証する。クライアントは暗号化されていない接続で
// パスワードを提示しないため、STARTTLS上で実行する。
func TestSMTPSender_Send_Authenticates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		username     string
		password     string
		wantAttempts int
	}{
		{name: "認証情報あり", username: "smtp-user", password: "smtp-password", wantAttempts: 1},
		{name: "認証情報なし", username: "", password: "", wantAttempts: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := newFakeSMTPServer(t, false, true)
			sender := newTestSMTPSender(t, server, SMTPTLSModeStartTLS, tt.username, tt.password)

			if err := sender.Send(context.Background(), testSendInput()); err != nil {
				t.Fatalf("Send()のエラー = %v", err)
			}

			if got := server.authenticated(); len(got) != tt.wantAttempts {
				t.Errorf("authenticated() = %v、期待値 = %d 回の試行", got, tt.wantAttempts)
			}
		})
	}
}

// TestSMTPSender_Send_UnencryptedWithCredentialsは、暗号化しないモードで
// パスワードを設定したときの挙動を固定する。クライアントはリレーが広告する方式を
// 1つも提示できないため送信が失敗し、何も配送されない。これを固定しておくことで、
// ライブラリの更新によってこの組み合わせが平文接続上のPLAINへ黙って変わることを防ぐ。
func TestSMTPSender_Send_UnencryptedWithCredentials(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeNone, "smtp-user", "smtp-password")

	if err := sender.Send(context.Background(), testSendInput()); err == nil {
		t.Fatal("Send()の失敗を期待したが、成功した (暗号化されていない接続でパスワードを提示すべきでない)")
	}

	if got := server.authenticated(); len(got) != 0 {
		t.Errorf("authenticated() = %v、期待値は試行なし", got)
	}
	if len(server.received()) != 0 {
		t.Error("使える認証方式が無いのにメッセージが配送された")
	}
}

// TestSMTPSender_Send_TLSModesは保護された各モードでの配送を、そのモードを話す
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
				t.Fatalf("Send()のエラー = %v", err)
			}

			if len(server.received()) != 1 {
				t.Fatalf("len(received()) = %d、期待値 = 1", len(server.received()))
			}
			if got := server.authenticated(); len(got) != 1 {
				t.Errorf("authenticated() = %v、期待値 = 1回の試行", got)
			}
		})
	}
}

// TestSMTPSender_Send_StartTLSRequiredは、STARTTLSモードがSTARTTLSを提供
// しないリレーに対して、暗号化されない接続へ黙ってフォールバックせず配送を拒否することを
// 検証する。
func TestSMTPSender_Send_StartTLSRequired(t *testing.T) {
	t.Parallel()

	server := newFakeSMTPServer(t, false, false)
	sender := newTestSMTPSender(t, server, SMTPTLSModeStartTLS, "", "")

	if err := sender.Send(context.Background(), testSendInput()); err == nil {
		t.Fatal("リレーがSTARTTLSを提供しないのにSend()が成功した")
	}
	if len(server.received()) != 0 {
		t.Error("STARTTLSが使えないのにメッセージが配送された")
	}
}

// TestSMTPSender_clientOptions_TLSPolicyは各モードが生成するポリシーを固定する。
// 3つの定数以外のモードのフォールバックも含む。
func TestSMTPSender_clientOptions_TLSPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode SMTPTLSMode
		want string
	}{
		{name: "starttls", mode: SMTPTLSModeStartTLS, want: mail.TLSMandatory.String()},
		{name: "none", mode: SMTPTLSModeNone, want: mail.NoTLS.String()},
		{name: "未知のモードはmandatoryにフォールバックする", mode: SMTPTLSMode("bogus"), want: mail.TLSMandatory.String()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sender := NewSMTPSender(SMTPConfig{Host: "smtp.example.dev", Port: 587, TLSMode: tt.mode})
			client, err := mail.NewClient(sender.config.Host, sender.clientOptions()...)
			if err != nil {
				t.Fatalf("mail.NewClient()のエラー = %v", err)
			}
			if got := client.TLSPolicy(); got != tt.want {
				t.Errorf("TLSPolicy() = %q、期待値 = %q", got, tt.want)
			}
		})
	}
}

// TestNewSMTPSender_TLSConfigはSenderが最小TLSバージョンを固定し、設定された
// ホスト名でリレーの証明書を検証することを確認する。
func TestNewSMTPSender_TLSConfig(t *testing.T) {
	t.Parallel()

	sender := NewSMTPSender(SMTPConfig{Host: "smtp.example.dev", Port: 587})

	if sender.tlsConfig.ServerName != "smtp.example.dev" {
		t.Errorf("ServerName = %q、期待値 = %q", sender.tlsConfig.ServerName, "smtp.example.dev")
	}
	if sender.tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = %d、期待値 = %d", sender.tlsConfig.MinVersion, tls.VersionTLS12)
	}
	if sender.tlsConfig.InsecureSkipVerify {
		t.Error("証明書の検証が無効になっている")
	}
}
