package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/wneessen/go-mail"
)

// SMTPTLSModeはSMTP接続の保護方式を選択する。
type SMTPTLSMode string

const (
	// SMTPTLSModeStartTLSは平文接続をSTARTTLSで昇格させ、サーバーが
	// STARTTLSを提供しない場合は送信を拒否する。
	SMTPTLSModeStartTLS SMTPTLSMode = "starttls"

	// SMTPTLSModeImplicitは接続を最初のバイトからTLSで包む。TLS上の
	// submissionポート (465) が前提とする方式。
	SMTPTLSModeImplicit SMTPTLSMode = "implicit"

	// SMTPTLSModeNoneは暗号化しない接続で送信する。信頼できるローカル経路で
	// 到達するリレー (同一ホスト上のMTAなど) のために用意する。本文が平文で流れるため、
	// ネットワークを越える経路では使ってはならない。リレーがチャレンジレスポンス方式を
	// 広告しない限りこのモードで認証情報は提示しない。PLAIN / LOGINしか提示しない
	// リレーに対しては、パスワードを設定してあると平文で流れる代わりに配送が失敗する。
	// 送信元アドレスで認可するリレーでは認証情報を未設定にすること。
	SMTPTLSModeNone SMTPTLSMode = "none"
)

// smtpTimeoutは1回の送信試行 (接続・ハンドシェイク・SMTPのやり取り) を
// 制限する。ResendのSenderがHTTPクライアントに設定するタイムアウトに揃え、
// どちらのtransportもworkerのgoroutineを無期限にブロックしないようにする。
const smtpTimeout = 30 * time.Second

// SMTPConfigはSMTPSenderが配送に使うSMTPリレーを表す。コンストラクタの
// 引数ではなく構造体にするのは、7項目のうち5つがstringであり、位置引数では
// コンパイラが取り違えを検出できないため。
type SMTPConfig struct {
	// HostとPortはリレーの宛先。
	Host string
	Port int

	// UsernameとPasswordはリレーへの認証情報。Usernameが空のときは認証を
	// 行わない。送信元アドレスで認可するリレーはこの形になる。
	Username string
	Password string

	// TLSModeは接続の保護方式を選択する。
	TLSMode SMTPTLSMode

	// FromEmailとFromNameはFromヘッダーを組み立てる。ResendのSenderの
	// 設定方法に揃えている。
	FromEmail string
	FromName  string
}

// SMTPSenderはSMTPリレー経由でメールを送信する。セルフホストされた
// インスタンス向けのSenderの本番実装であり、運用者は既に運用しているプロバイダーや
// MTAをGroobbから指定できる。
type SMTPSender struct {
	config SMTPConfig

	// tlsConfigはSTARTTLSによる昇格とimplicit TLSの接続の双方に適用する。
	// 最小プロトコルバージョンをライブラリの既定に委ねず固定することで、リレー側が
	// 非推奨のTLSバージョンへ接続を引き下げられないようにする。
	tlsConfig *tls.Config
}

// NewSMTPSenderは指定されたリレー向けのSMTPSenderを構築する。
func NewSMTPSender(config SMTPConfig) *SMTPSender {
	return &SMTPSender{
		config: config,
		tlsConfig: &tls.Config{
			ServerName: config.Host,
			MinVersion: tls.VersionTLS12,
		},
	}
}

// Sendは本文をレンダリングし、設定されたリレー経由でメールを配送する。
func (s *SMTPSender) Send(ctx context.Context, input SendInput) error {
	message, err := s.buildMessage(ctx, input)
	if err != nil {
		return err
	}

	client, err := mail.NewClient(s.config.Host, s.clientOptions()...)
	if err != nil {
		return fmt.Errorf("SMTPクライアントの作成に失敗: %w", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.DialAndSendWithContext(ctx, message); err != nil {
		return fmt.Errorf("メール送信に失敗: %w", err)
	}
	return nil
}

// buildMessageはtemplの本文を1通のmultipart/alternativeメッセージへ
// レンダリングする。
//
// テキストを本文、HTMLをalternativeとして設定することで、この順序で並ぶ。RFC 2046は
// クライアントが描画できる最後のパートを表示すると定めており、HTMLを後ろに置くことが
// HTMLを優先して表示させる条件になる。
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
		return nil, fmt.Errorf("HTML本文のレンダリングに失敗: %w", err)
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

// clientOptionsは設定をSMTPクライアントが受け取るオプションへ変換する。
//
// 3つの定数以外のモードはライブラリの既定ではなくSTARTTLSへフォールバックさせる。
// ライブラリの既定は日和見的に交渉し、リレーがSTARTTLSを提供しない場合は暗号化
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

	// SMTPAuthAutoDiscoverはリレーが広告する中で最も強い方式を選ぶ。これにより、
	// 受け付ける方式がプロバイダーごとに異なっても1つの設定で通る。
	if s.config.Username != "" {
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.config.Username),
			mail.WithPassword(s.config.Password),
		)
	}

	return options
}
