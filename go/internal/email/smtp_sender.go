package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPTLSMode selects how an SMTP connection is secured.
//
// [Ja] SMTPTLSMode は SMTP 接続の保護方式を選択する。
type SMTPTLSMode string

const (
	// SMTPTLSModeStartTLS upgrades a plaintext connection with STARTTLS and
	// refuses to send when the server does not offer it.
	//
	// [Ja] SMTPTLSModeStartTLS は平文接続を STARTTLS で昇格させ、サーバーが
	// STARTTLS を提供しない場合は送信を拒否する。
	SMTPTLSModeStartTLS SMTPTLSMode = "starttls"

	// SMTPTLSModeImplicit wraps the connection in TLS from the first byte, as
	// the submission-over-TLS port (465) expects.
	//
	// [Ja] SMTPTLSModeImplicit は接続を最初のバイトから TLS で包む。TLS 上の
	// submission ポート (465) が前提とする方式。
	SMTPTLSModeImplicit SMTPTLSMode = "implicit"

	// SMTPTLSModeNone sends over an unencrypted connection. It exists for a relay
	// reached over a trusted local channel (a mail transfer agent on the same
	// host); anything crossing a network must not use it, because the message
	// travels in the clear. Credentials are not offered over this mode unless the
	// relay advertises a challenge-response mechanism; against a relay that only
	// offers PLAIN or LOGIN, a configured password makes delivery fail rather than
	// travel in the clear. Leave the credentials unset for a relay that authorises
	// by source address.
	//
	// [Ja] SMTPTLSModeNone は暗号化しない接続で送信する。信頼できるローカル経路で
	// 到達するリレー (同一ホスト上の MTA など) のために用意する。本文が平文で流れるため、
	// ネットワークを越える経路では使ってはならない。リレーがチャレンジレスポンス方式を
	// 広告しない限りこのモードで認証情報は提示しない。PLAIN / LOGIN しか提示しない
	// リレーに対しては、パスワードを設定してあると平文で流れる代わりに配送が失敗する。
	// 送信元アドレスで認可するリレーでは認証情報を未設定にすること。
	SMTPTLSModeNone SMTPTLSMode = "none"
)

// smtpTimeout bounds a single delivery attempt (connect, handshake, and the SMTP
// exchange). It mirrors the timeout the Resend sender puts on its HTTP client so
// that neither transport can block a worker goroutine indefinitely.
//
// [Ja] smtpTimeout は 1 回の送信試行 (接続・ハンドシェイク・SMTP のやり取り) を
// 制限する。Resend の Sender が HTTP クライアントに設定するタイムアウトに揃え、
// どちらの transport も worker の goroutine を無期限にブロックしないようにする。
const smtpTimeout = 30 * time.Second

// SMTPConfig describes the SMTP relay a SMTPSender delivers through. It is a
// struct rather than constructor arguments because five of the seven settings
// are strings, which the compiler cannot tell apart when they are positional.
//
// [Ja] SMTPConfig は SMTPSender が配送に使う SMTP リレーを表す。コンストラクタの
// 引数ではなく構造体にするのは、7 項目のうち 5 つが string であり、位置引数では
// コンパイラが取り違えを検出できないため。
type SMTPConfig struct {
	// Host and Port address the relay.
	//
	// [Ja] Host と Port はリレーの宛先。
	Host string
	Port int

	// Username and Password authenticate to the relay. When Username is empty no
	// authentication is attempted, which suits a relay that authorises by source
	// address instead.
	//
	// [Ja] Username と Password はリレーへの認証情報。Username が空のときは認証を
	// 行わない。送信元アドレスで認可するリレーはこの形になる。
	Username string
	Password string

	// TLSMode selects how the connection is secured.
	//
	// [Ja] TLSMode は接続の保護方式を選択する。
	TLSMode SMTPTLSMode

	// FromEmail and FromName build the From header, matching how the Resend
	// sender is configured.
	//
	// [Ja] FromEmail と FromName は From ヘッダーを組み立てる。Resend の Sender の
	// 設定方法に揃えている。
	FromEmail string
	FromName  string
}

// SMTPSender sends email through an SMTP relay. It is the production
// implementation of Sender for a self-hosted instance, where the operator points
// Groobb at whichever provider or mail transfer agent they already run.
//
// [Ja] SMTPSender は SMTP リレー経由でメールを送信する。セルフホストされた
// インスタンス向けの Sender の本番実装であり、運用者は既に運用しているプロバイダーや
// MTA を Groobb から指定できる。
type SMTPSender struct {
	config SMTPConfig

	// tlsConfig is applied to both the STARTTLS upgrade and the implicit-TLS
	// dial. It pins a minimum protocol version rather than leaving it to the
	// library default, so a relay cannot negotiate the connection down to a
	// deprecated version of TLS.
	//
	// [Ja] tlsConfig は STARTTLS による昇格と implicit TLS の接続の双方に適用する。
	// 最小プロトコルバージョンをライブラリの既定に委ねず固定することで、リレー側が
	// 非推奨の TLS バージョンへ接続を引き下げられないようにする。
	tlsConfig *tls.Config
}

// NewSMTPSender builds an SMTPSender for the given relay.
//
// [Ja] NewSMTPSender は指定されたリレー向けの SMTPSender を構築する。
func NewSMTPSender(config SMTPConfig) *SMTPSender {
	return &SMTPSender{
		config: config,
		tlsConfig: &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		},
	}
}

// Send renders the bodies and delivers the email through the configured relay.
//
// [Ja] Send は本文をレンダリングし、設定されたリレー経由でメールを配送する。
func (s *SMTPSender) Send(ctx context.Context, input SendInput) error {
	message, err := s.buildMessage(ctx, input)
	if err != nil {
		return err
	}

	client, err := mail.NewClient(s.config.Host, s.clientOptions()...)
	if err != nil {
		return fmt.Errorf("SMTP クライアントの作成に失敗: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("メール送信に失敗: %w", err)
	}
	return nil
}

// buildMessage renders the templ bodies into one multipart/alternative message.
//
// The plain-text part is set as the body and the HTML part as the alternative so
// that the two land in that order: RFC 2046 has a client display the last part it
// can render, so putting HTML last is what makes it the preferred rendering.
//
// [Ja] buildMessage は templ の本文を 1 通の multipart/alternative メッセージへ
// レンダリングする。
//
// テキストを本文、HTML を alternative として設定することで、この順序で並ぶ。RFC 2046 は
// クライアントが描画できる最後のパートを表示すると定めており、HTML を後ろに置くことが
// HTML を優先して表示させる条件になる。
func (s *SMTPSender) buildMessage(ctx context.Context, input SendInput) (*mail.Msg, error) {
	message := mail.NewMsg()
	if err := message.FromFormat(s.config.FromName, s.config.FromEmail); err != nil {
		return nil, fmt.Errorf("送信元アドレスの設定に失敗: %w", err)
	}
	if err := message.To(input.To); err != nil {
		return nil, fmt.Errorf("宛先アドレスの設定に失敗: %w", err)
	}
	message.Subject(input.Subject)

	var htmlBuf bytes.Buffer
	if err := input.HTMLBody.Render(ctx, &htmlBuf); err != nil {
		return nil, fmt.Errorf("HTML 本文のレンダリングに失敗: %w", err)
	}

	if input.TextBody == nil {
		message.SetBodyString(mail.TypeTextHTML, htmlBuf.String())
		return message, nil
	}

	var textBuf bytes.Buffer
	if err := input.TextBody.Render(ctx, &textBuf); err != nil {
		return nil, fmt.Errorf("テキスト本文のレンダリングに失敗: %w", err)
	}
	message.SetBodyString(mail.TypeTextPlain, textBuf.String())
	message.AddAlternativeString(mail.TypeTextHTML, htmlBuf.String())
	return message, nil
}

// clientOptions translates the configuration into the options the SMTP client
// takes.
//
// A mode outside the three constants falls back to STARTTLS rather than to the
// library default: the library default negotiates opportunistically and delivers
// over an unencrypted connection when the relay withholds STARTTLS, which would
// turn a typo in the setting into silent plaintext delivery.
//
// [Ja] clientOptions は設定を SMTP クライアントが受け取るオプションへ変換する。
//
// 3 つの定数以外のモードはライブラリの既定ではなく STARTTLS へフォールバックさせる。
// ライブラリの既定は日和見的に交渉し、リレーが STARTTLS を提供しない場合は暗号化
// されない接続で配送するため、設定のタイプミスが黙って平文配送に変わってしまう。
func (s *SMTPSender) clientOptions() []mail.Option {
	options := []mail.Option{
		mail.WithPort(s.config.Port),
		mail.WithTimeout(smtpTimeout),
		mail.WithTLSConfig(s.tlsConfig),
	}

	switch s.config.TLSMode {
	case SMTPTLSModeImplicit:
		options = append(options, mail.WithSSL())
	case SMTPTLSModeNone:
		options = append(options, mail.WithTLSPolicy(mail.NoTLS))
	default:
		options = append(options, mail.WithTLSPolicy(mail.TLSMandatory))
	}

	// SMTPAuthAutoDiscover picks the strongest mechanism the relay advertises,
	// which is what lets one setting work across providers that each accept a
	// different subset.
	//
	// [Ja] SMTPAuthAutoDiscover はリレーが広告する中で最も強い方式を選ぶ。これにより、
	// 受け付ける方式がプロバイダーごとに異なっても 1 つの設定で通る。
	if s.config.Username != "" {
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.config.Username),
			mail.WithPassword(s.config.Password),
		)
	}

	return options
}
