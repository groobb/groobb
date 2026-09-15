package viewmodel

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/model"
)

// PostBodyTokenKindは投稿本文の断片どうしを区別します。テンプレートが、それが何で
// あるかを知るためにテキストをもう一度読むのではなく、そのものとして描画できるように
// するためです。
type PostBodyTokenKind int

const (
	// PostBodyTextは、それ自身を表す本文の断片です。ゼロ値であるため、どの規則
	// にも一致しない本文は1つのテキストになります。
	PostBodyText PostBodyTokenKind = iota

	// PostBodyPostReferenceは、同じスレッドが持つ投稿を名指す >>Nです。持たない
	// 投稿を名指す >>Nはテキストです。この慣習がリンクになるのは、連れて行く先がある
	// ときだけです。
	PostBodyPostReference

	// PostBodyURLは本文に書かれたウェブアドレスです。
	PostBodyURL
)

// PostBodyTokenは投稿本文の断片1つと、テンプレートがそれを何として描画するかです。
// 区別の付いた状態で渡すのは、テキストの断片が何であるかを決めるのがマークアップではなく
// Goの仕事だからです。参照を認識せねばならないテンプレートは、それを2度目に、しかも
// 保存する側からは見えない場所に置かれた規則で認識することになります。
type PostBodyToken struct {
	Kind PostBodyTokenKind

	// Textはこのトークンが表す本文の断片で、書かれたままの形です。どの種類も
	// これを運び、どの種類もこれを描画します。リンクになる2種類も同じで、書き手が
	// 打った文字がそのままラベルになります。レス番号やアドレスについて、それ以上に
	// うまく述べるものが他に無いためです。
	Text string

	// NumberはPostBodyPostReferenceが名指すレス番号で、他の種類では0です。
	// リンクを組み立てるのはTextの中の数字ではなくこちらです。>>007が >>7と同じ
	// 場所へ繋がるようにするためです。
	Number int
}

// PostBodyは、テンプレートが描画する形の投稿本文、すなわちテキストの断片・レス
// 参照・アドレスへと分解した本文です。
//
// 分解をテンプレートではなくここで行うのは、そうしない場合の道が、HTMLを文字列として
// 組み立てて生のマークアップとして渡すことになるためです。それは訪問者が書いたもの
// すべてのエスケープを、次にその文字列を編集する人の手に委ねます。断片として渡せば、
// そのひとつひとつをtemplがエスケープし続けます。
type PostBody struct {
	Tokens []PostBodyToken
}

// postBodyURLPatternは、本文に書かれたままのウェブアドレスに一致します。ページを
// 取得できる2つのスキームのいずれかと、それに続くアドレスを書くための文字です。
//
// 一致するのはその2つのスキームだけであるため、本文からhrefへ到達する他のスキームは
// ありません。スキームは大文字小文字を区別しないため、どちらの綴り方で書かれていても
// 一致させます。その後ろに続くのはURLを構成するASCII文字 (RFC 3986) であり、これが、
// 周囲の文が再開する場所でアドレスを終わらせます。日本語は空白を置かずに続くため、そう
// しなければ「`…/help を見て`」と書かれた本文と「`…/helpを見て`」と書かれた本文が、書き手が
// 意識してもいない空白によって区別されることになります。これはまた、アドレスの直後に
// 書かれた >>Nをその外側に残します。
//
// 代償は、パーセントエンコードせずに文字そのもので書かれたアドレスの、ASCIIでない部分が
// リンクから外れることです。2つのうちではこちらが稀です。アドレスは打たれるよりはるかに
// 多く貼り付けられ、ブラウザがクリップボードへ渡すものはパーセントエンコード済みです。
var postBodyURLPattern = regexp.MustCompile(`(?i:https?)://[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)

// postBodyURLTailは、アドレスそのものではなく、アドレスが書き込まれた文を閉じる
// 文字を持ちます。文の途中に書かれたアドレスは、間に空白を挟まずに後続の句読点と接する
// ため、一致した範囲はそれらの文字を周囲のテキストへ返さねばなりません。
//
// 丸括弧と角括弧の閉じ括弧が意図的にこの集合から外れているのは、どちらもアドレスの一部で
// あることも周囲のテキストの終わりであることもあるためです。どちらであるかは、アドレス
// 自身が対応する括弧を開いているかどうかで判別でき、それを見るのがtrimPostBodyURLです。
const postBodyURLTail = `.,:;!?'`

// NewPostBodyはbodyを、テンプレートがそれを描画するための断片へ分解します。
//
// postNumbersはスレッドが持つレス番号の集合です。そこに含まれる >>Nはその投稿への
// リンクになり、含まれない >>Nはテキストのままになります。レス番号が意味を持つ場所は
// スレッドであり、どの番号がそこにあるのかは本文だけでは分からないためです。
func NewPostBody(body string, postNumbers map[int]bool) PostBody {
	marks := postBodyMarks(body, postNumbers)
	if len(marks) == 0 {
		if body == "" {
			return PostBody{}
		}

		return PostBody{Tokens: []PostBodyToken{{Kind: PostBodyText, Text: body}}}
	}

	tokens := make([]PostBodyToken, 0, 2*len(marks)+1)
	text := 0
	for _, mark := range marks {
		if mark.start > text {
			tokens = append(tokens, PostBodyToken{Kind: PostBodyText, Text: body[text:mark.start]})
		}

		tokens = append(tokens, mark.token)
		text = mark.start + len(mark.token.Text)
	}
	if text < len(body) {
		tokens = append(tokens, PostBodyToken{Kind: PostBodyText, Text: body[text:]})
	}

	return PostBody{Tokens: tokens}
}

// postBodyMarkは、ただのテキスト以外のものとして描画される本文の断片1つと、
// それが本文のどこから始まるかです。長さはトークンのテキストの長さそのものであるため、
// markとそれが表す断片が食い違うことはありません。
type postBodyMark struct {
	start int
	token PostBodyToken
}

// postBodyMarksは、bodyの中でテキスト以上のものとして描画されるものを、書かれた
// 順にすべて見つけます。
//
// 2つの走査が重なることはありません。アドレスが > を含むことはなく (postBodyURLPattern
// を参照)、参照は必ずそれで始まるためです。したがって開始位置で並べるだけで、結果を
// 隙間なく並べられます。
func postBodyMarks(body string, postNumbers map[int]bool) []postBodyMark {
	var marks []postBodyMark

	for _, span := range model.PostReferenceSpans(body) {
		if !postNumbers[span.Number] {
			continue
		}

		marks = append(marks, postBodyMark{
			start: span.Start,
			token: PostBodyToken{
				Kind:   PostBodyPostReference,
				Text:   body[span.Start:span.End],
				Number: span.Number,
			},
		})
	}

	for _, match := range postBodyURLPattern.FindAllStringIndex(body, -1) {
		url := trimPostBodyURL(body[match[0]:match[1]])

		// 切り詰めた結果、スキームだけが残ることがあります。その後ろにあったものが
		// 文を閉じる句読点だった場合です。それは何も指さないため、読んだとおりの
		// テキストへ戻します。
		if _, address, _ := strings.Cut(url, "://"); address == "" {
			continue
		}

		marks = append(marks, postBodyMark{
			start: match[0],
			token: PostBodyToken{Kind: PostBodyURL, Text: url},
		})
	}

	slices.SortFunc(marks, func(a, b postBodyMark) int { return cmp.Compare(a.start, b.start) })

	return marks
}

// trimPostBodyURLは、アドレスの周りの文がその後ろに置いた句読点を返し、アドレス
// 自身を残します。
//
// 丸括弧または角括弧の閉じ括弧が周囲のテキストのものであるのは、アドレス自身に対応する
// 閉じられていない開き括弧が無いときだけです。これにより、括弧の対応するアドレスはそれを
// 保ち、括弧の中に書かれたアドレスは外側の閉じ括弧を取り込みません。
func trimPostBodyURL(url string) string {
	for url != "" {
		r, size := utf8.DecodeLastRuneInString(url)

		sentences := strings.ContainsRune(postBodyURLTail, r) ||
			(r == ')' && strings.Count(url, ")") > strings.Count(url, "(")) ||
			(r == ']' && strings.Count(url, "]") > strings.Count(url, "["))
		if !sentences {
			return url
		}

		url = url[:len(url)-size]
	}

	return url
}
