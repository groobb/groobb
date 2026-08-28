package model

import "regexp"

// SlugMaxLength is the maximum length of a slug, in characters. A slug is
// restricted to ASCII (see slugRegex), so the byte length equals the character
// count for any value that passes the format check.
//
// [Ja] SlugMaxLength は slug の最大文字数。slug は ASCII に限定される (slugRegex 参照)
// ため、形式チェックを通る値ではバイト長と文字数が一致する。
const SlugMaxLength = 30

// slugRegex is the allowed slug format: one or more lowercase ASCII letters,
// digits, hyphens, or underscores. Lowercase gives every category and board one
// canonical URL. Every character it admits stands for itself in a URL path, so a
// slug that passes is placed after /c/ or /b/ as written, without percent-encoding.
// It requires at least one character, so an empty slug fails it.
//
// [Ja] slugRegex は許可する slug の形式: 1 文字以上の ASCII 英小文字・数字・ハイフン・
// アンダースコア。小文字に揃えることで各カテゴリーと掲示板の正規 URL を 1 つにします。
// ここで許す文字はいずれも URL のパスの中でそれ自身を表すため、通過した slug は
// パーセントエンコードを挟まずそのまま /c/ や /b/ の後ろに置けます。1 文字以上を要求する
// ため空の slug は不適合になります。
var slugRegex = regexp.MustCompile(`^[a-z0-9_-]+$`)

// IsValidSlug reports whether slug is one a category or a board can be
// addressed by: within SlugMaxLength and matching the allowed format.
//
// The rule lives beside the entities it constrains rather than beside a caller,
// because the address a slug produces is built elsewhere
// (templates.CategoryPath, templates.BoardPath) and assumes a value this has
// accepted. A second copy of the rule would let the two drift apart, and a slug
// carrying a path or query character would then produce a link pointing
// somewhere other than the board.
//
// It sits in this package rather than in internal/validator so that the
// repositories that insert categories and boards can apply it: validator reads
// through repository for its state checks, so a repository importing validator
// would close a cycle. What a slug may be is a rule about the category and the
// board themselves, which is what this package holds.
//
// [Ja] IsValidSlug は、slug がカテゴリーや掲示板を指せるもの (SlugMaxLength 以内で、
// 許可された形式に適合するもの) かどうかを返します。
//
// 規則を呼び出し元の隣ではなく、それが制約する対象の隣に置くのは、slug が作るアドレスを
// 組み立てるのが別の場所 (templates.CategoryPath・templates.BoardPath) であり、そこが
// 本関数を通った値を前提にしているためです。規則の写しが 2 つあれば両者は離れていき、
// パスやクエリの文字を含む slug が掲示板ではないどこかを指すリンクを作ることになります。
//
// internal/validator ではなく本パッケージに置くのは、カテゴリーと掲示板を挿入する
// リポジトリがこれを適用できるようにするためです。validator は状態の検査のために
// repository を読むため、repository が validator を import すると循環します。slug が
// どのような値でありうるかはカテゴリーと掲示板そのものについての規則であり、それは
// 本パッケージが持つものです。
func IsValidSlug(slug string) bool {
	return len(slug) <= SlugMaxLength && slugRegex.MatchString(slug)
}
