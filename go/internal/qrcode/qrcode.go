// qrcodeパッケージは、HTMLテンプレートに直接埋め込むために内容をQRコードの
// PNG data URIとしてレンダリングします。
//
// これはPresentation層のヘルパーです。文字列 (TOTPのotpauth URIなど) を自己完結した
// data: URIに変換し、テンプレートが素の <img> で追加のHTTPリクエストなしにQRコードを
// 表示できるようにします。標準ライブラリと外部のQRライブラリのみに依存し、Groobbの他
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

// pixelSizeはレンダリングするQR画像の幅と高さ (ピクセル) です。256 pxは画面上で
// 確実にスキャンできる大きさでありながら、data URIを小さく保てます。
const pixelSize = 256

// PNGDataURIはcontentをQRコードにエンコードし、<img src> にそのまま入れられる
// "data:image/png;base64,..." 形式のURIとして返します。otpauth URIのような短い文字列に
// 対して、耐性と密度のバランスが良い中程度の誤り訂正を用います。
func PNGDataURI(content string) (string, error) {
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return "", fmt.Errorf("QRコードのエンコードに失敗: %w", err)
	}
	scaled, err := barcode.Scale(code, pixelSize, pixelSize)
	if err != nil {
		return "", fmt.Errorf("QRコードのスケーリングに失敗: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("QRコードのPNGエンコードに失敗: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
