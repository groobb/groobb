// Package qrcode renders content as a QR code PNG data URI for embedding directly
// in an HTML template.
//
// It is a Presentation-layer helper: it turns a string (such as a TOTP otpauth
// URI) into a self-contained data: URI so a template can show the QR code with a
// plain <img> and no extra HTTP request. It depends only on the standard library
// and an external QR library, never on other Groobb packages.
//
// [Ja] qrcode パッケージは、HTML テンプレートに直接埋め込むために内容を QR コードの
// PNG data URI としてレンダリングします。
//
// これは Presentation 層のヘルパーです。文字列 (TOTP の otpauth URI など) を自己完結した
// data: URI に変換し、テンプレートが素の <img> で追加の HTTP リクエストなしに QR コードを
// 表示できるようにします。標準ライブラリと外部の QR ライブラリのみに依存し、Groobb の他
// パッケージには依存しません。
package qrcode

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
)

// pixelSize is the width and height in pixels of the rendered QR image. 256 px is
// large enough to scan reliably on screen while keeping the data URI small.
//
// [Ja] pixelSize はレンダリングする QR 画像の幅と高さ (ピクセル) です。256 px は画面上で
// 確実にスキャンできる大きさでありながら、data URI を小さく保てます。
const pixelSize = 256

// PNGDataURI encodes content as a QR code and returns it as a
// "data:image/png;base64,..." URI ready to drop into an <img src>. It uses medium
// error correction, a good balance of resilience and density for a short string
// like an otpauth URI.
//
// [Ja] PNGDataURI は content を QR コードにエンコードし、<img src> にそのまま入れられる
// "data:image/png;base64,..." 形式の URI として返します。otpauth URI のような短い文字列に
// 対して、耐性と密度のバランスが良い中程度の誤り訂正を用います。
func PNGDataURI(content string) (string, error) {
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("QR コードのエンコードに失敗: %w", err)
	}
	scaled, err := barcode.Scale(code, pixelSize, pixelSize)
	if err != nil {
		return "", fmt.Errorf("QR コードのスケーリングに失敗: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("QR コードの PNG エンコードに失敗: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
