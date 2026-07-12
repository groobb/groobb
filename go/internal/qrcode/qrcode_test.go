package qrcode_test

import (
	"encoding/base64"
	"image/png"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/qrcode"
)

const dataURIPrefix = "data:image/png;base64,"

// TestPNGDataURI verifies that the result is a base64 PNG data URI whose payload
// decodes to a valid 256x256 PNG, so a template can embed it directly in an
// <img src>.
//
// [Ja] TestPNGDataURI は、結果が base64 の PNG data URI であり、そのペイロードが有効な
// 256x256 の PNG にデコードされることを検証します。テンプレートが <img src> に直接
// 埋め込めるようにするためです。
func TestPNGDataURI(t *testing.T) {
	t.Parallel()

	uri, err := qrcode.PNGDataURI("otpauth://totp/Groobb:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Groobb")
	if err != nil {
		t.Fatalf("PNGDataURI() error = %v", err)
	}

	payload, ok := strings.CutPrefix(uri, dataURIPrefix)
	if !ok {
		t.Fatalf("PNGDataURI() = %q, want it to start with %q", uri, dataURIPrefix)
	}

	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("data URI payload is not valid base64: %v", err)
	}

	img, err := png.Decode(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("data URI payload is not a valid PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("PNG size = %dx%d, want 256x256", b.Dx(), b.Dy())
	}
}

// TestPNGDataURI_Deterministic verifies that the same content always encodes to
// the same data URI, since QR encoding is deterministic.
//
// [Ja] TestPNGDataURI_Deterministic は、QR エンコードが決定的であるため、同じ content が
// 常に同じ data URI にエンコードされることを検証します。
func TestPNGDataURI_Deterministic(t *testing.T) {
	t.Parallel()

	const content = "otpauth://totp/Groobb:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Groobb"

	first, err := qrcode.PNGDataURI(content)
	if err != nil {
		t.Fatalf("PNGDataURI() error = %v", err)
	}
	second, err := qrcode.PNGDataURI(content)
	if err != nil {
		t.Fatalf("PNGDataURI() error = %v", err)
	}
	if first != second {
		t.Error("PNGDataURI() が同じ content に対して異なる出力を返した (同一であるべき)")
	}
}
