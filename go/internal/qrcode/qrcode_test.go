package qrcode_test

import (
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/qrcode"
)

const dataURIPrefix = "data:image/png;base64,"

// TestPNGDataURIは、結果がbase64のPNG data URIであり、そのペイロードが有効な
// 256x256のPNGにデコードされることを検証します。テンプレートが <img src> に直接
// 埋め込めるようにするためです。
func TestPNGDataURI(t *testing.T) {
	t.Parallel()

	uri, err := qrcode.PNGDataURI("otpauth://totp/Groobb:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Groobb")
	if err != nil {
		t.Fatalf("PNGDataURI()のエラー = %v", err)
	}

	payload, ok := strings.CutPrefix(uri, dataURIPrefix)
	if !ok {
		t.Fatalf("PNGDataURI() = %q、%q で始まることを期待", uri, dataURIPrefix)
	}

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("data URIのペイロードが正しいbase64でない: %v", err)
	}

	img, err := png.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("data URIのペイロードが正しいPNGでない: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("PNGのサイズ = %dx%d、期待値 = 256x256", b.Dx(), b.Dy())
	}
}

// TestPNGDataURI_Deterministicは、QRエンコードが決定的であるため、同じcontentが
// 常に同じdata URIにエンコードされることを検証します。
func TestPNGDataURI_Deterministic(t *testing.T) {
	t.Parallel()

	const content = "otpauth://totp/Groobb:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Groobb"

	first, err := qrcode.PNGDataURI(content)
	if err != nil {
		t.Fatalf("PNGDataURI()のエラー = %v", err)
	}
	second, err := qrcode.PNGDataURI(content)
	if err != nil {
		t.Fatalf("PNGDataURI()のエラー = %v", err)
	}
	if first != second {
		t.Error("PNGDataURI()が同じcontentに対して異なる出力を返した (同一であるべき)")
	}
}
