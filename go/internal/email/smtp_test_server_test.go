package email

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTPServer is a minimal SMTP server that records what a client sent it. It
// exists so the sender can be exercised over a real socket, including the TLS
// handshake and the AUTH exchange, which a stubbed client interface would leave
// untested.
//
// [Ja] fakeSMTPServer はクライアントが送ってきた内容を記録する最小の SMTP サーバー。
// TLS ハンドシェイクや AUTH のやり取りを含めて実際のソケット越しに Sender を動かすために
// 用意する。クライアントのインターフェースをスタブに差し替える方式ではこれらが未検証に
// なる。
type fakeSMTPServer struct {
	// implicitTLS wraps the connection in TLS before the greeting; offerStartTLS
	// advertises STARTTLS in the EHLO response.
	//
	// [Ja] implicitTLS は挨拶の前に接続を TLS で包む。offerStartTLS は EHLO の応答で
	// STARTTLS を広告する。
	implicitTLS    bool
	offerStartTLS  bool
	certificate    tls.Certificate
	listener       net.Listener
	mu             sync.Mutex
	messages       []string
	authMechanisms []string
	recipients     []string
}

// newFakeSMTPServer starts a server on a loopback port and stops it when the
// test ends.
//
// [Ja] newFakeSMTPServer はループバックのポートでサーバーを起動し、テスト終了時に
// 停止する。
func newFakeSMTPServer(t *testing.T, implicitTLS, offerStartTLS bool) *fakeSMTPServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("テスト用 SMTP サーバーの listen に失敗: %v", err)
	}

	server := &fakeSMTPServer{
		implicitTLS:   implicitTLS,
		offerStartTLS: offerStartTLS,
		certificate:   selfSignedCertificate(t),
		listener:      listener,
	}
	t.Cleanup(func() { _ = listener.Close() })

	go server.acceptLoop()

	return server
}

// hostPort returns the address the server listens on, split for SMTPConfig.
//
// [Ja] hostPort はサーバーの待ち受けアドレスを SMTPConfig 向けに分解して返す。
func (s *fakeSMTPServer) hostPort(t *testing.T) (string, int) {
	t.Helper()

	host, port, err := net.SplitHostPort(s.listener.Addr().String())
	if err != nil {
		t.Fatalf("待ち受けアドレスの分解に失敗: %v", err)
	}
	number, err := strconv.Atoi(port)
	if err != nil {
		t.Fatalf("ポート番号の解釈に失敗: %v", err)
	}
	return host, number
}

// clientTLSConfig trusts the server's self-signed certificate.
//
// [Ja] clientTLSConfig はサーバーの自己署名証明書を信頼する設定を返す。
func (s *fakeSMTPServer) clientTLSConfig(host string) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(s.certificate.Leaf)
	return &tls.Config{ServerName: host, RootCAs: pool, MinVersion: tls.VersionTLS12}
}

func (s *fakeSMTPServer) tlsConfig() *tls.Config {
	return &tls.Config{Certificates: []tls.Certificate{s.certificate}, MinVersion: tls.VersionTLS12}
}

func (s *fakeSMTPServer) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *fakeSMTPServer) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	if s.implicitTLS {
		tlsConn := tls.Server(conn, s.tlsConfig())
		if err := tlsConn.Handshake(); err != nil {
			return
		}
		conn = tlsConn
	}

	reader := bufio.NewReader(conn)
	reply := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	reply("220 fake ESMTP")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}

		command := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(command, "EHLO"):
			_, _ = conn.Write([]byte("250-fake\r\n250-AUTH PLAIN LOGIN\r\n"))
			if s.offerStartTLS {
				_, _ = conn.Write([]byte("250-STARTTLS\r\n"))
			}
			reply("250 8BITMIME")
		case strings.HasPrefix(command, "HELO"):
			reply("250 fake")
		case strings.HasPrefix(command, "STARTTLS"):
			reply("220 ready to start TLS")
			tlsConn := tls.Server(conn, s.tlsConfig())
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			reply = func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
		case strings.HasPrefix(command, "AUTH LOGIN"):
			s.append(&s.authMechanisms, "LOGIN")
			reply("334 " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			if _, err := reader.ReadString('\n'); err != nil {
				return
			}
			reply("334 " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			if _, err := reader.ReadString('\n'); err != nil {
				return
			}
			reply("235 authenticated")
		case strings.HasPrefix(command, "AUTH PLAIN"):
			s.append(&s.authMechanisms, "PLAIN")
			reply("235 authenticated")
		case strings.HasPrefix(command, "RCPT TO"):
			s.append(&s.recipients, strings.TrimSpace(line[len("RCPT TO:"):]))
			reply("250 ok")
		case strings.HasPrefix(command, "DATA"):
			reply("354 end with a dot")
			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				body.WriteString(dataLine)
			}
			s.append(&s.messages, body.String())
			reply("250 queued")
		case strings.HasPrefix(command, "QUIT"):
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func (s *fakeSMTPServer) append(destination *[]string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	*destination = append(*destination, value)
}

func (s *fakeSMTPServer) received() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.messages...)
}

func (s *fakeSMTPServer) authenticated() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.authMechanisms...)
}

func (s *fakeSMTPServer) rcptTo() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.recipients...)
}

// selfSignedCertificate issues a certificate for the loopback address so the
// TLS paths can be exercised without a certificate authority.
//
// [Ja] selfSignedCertificate はループバックアドレス向けの証明書を発行し、認証局なしで
// TLS の経路を動かせるようにする。
func selfSignedCertificate(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("テスト用鍵の生成に失敗: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("テスト用証明書の生成に失敗: %v", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("テスト用証明書の解析に失敗: %v", err)
	}

	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}
