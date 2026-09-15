package model

import "regexp"

// SlugMaxLengthはslugの最大文字数。slugはASCIIに限定される (slugRegex参照)
// ため、形式チェックを通る値ではバイト長と文字数が一致する。
const SlugMaxLength = 30

// slugRegexは許可するslugの形式: 1文字以上のASCII英小文字・数字・ハイフン・
// アンダースコア。小文字に揃えることで各カテゴリーと掲示板の正規URLを1つにします。
// ここで許す文字はいずれもURLのパスの中でそれ自身を表すため、通過したslugは
// パーセントエンコードを挟まずそのまま /c/ や /b/ の後ろに置けます。1文字以上を要求する
// ため空のslugは不適合になります。
var slugRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

// IsValidSlugは、slugがカテゴリーや掲示板を指せるもの (SlugMaxLength以内で、
// 許可された形式に適合するもの) かどうかを返します。
//
// 規則を呼び出し元の隣ではなく、それが制約する対象の隣に置くのは、slugが作るアドレスを
// 組み立てるのが別の場所 (templates.CategoryPath・templates.BoardPath) であり、そこが
// 本関数を通った値を前提にしているためです。規則の写しが2つあれば両者は離れていき、
// パスやクエリの文字を含むslugが掲示板ではないどこかを指すリンクを作ることになります。
//
// internal/validatorではなく本パッケージに置くのは、カテゴリーと掲示板を挿入する
// リポジトリがこれを適用できるようにするためです。validatorは状態の検査のために
// repositoryを読むため、repositoryがvalidatorをimportすると循環します。slugが
// どのような値でありうるかはカテゴリーと掲示板そのものについての規則であり、それは
// 本パッケージが持つものです。
func IsValidSlug(slug string) bool {
	return len(slug) <= SlugMaxLength && slugRegex.MatchString(slug)
}
