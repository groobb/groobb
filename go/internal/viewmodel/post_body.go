package viewmodel

import (
	"cmp"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/groobb/groobb/go/internal/model"
)

// PostBodyTokenKind tells one piece of a post body apart from the others, so
// that the template renders each as what it is instead of reading the text a
// second time to find out.
//
// [Ja] PostBodyTokenKind は投稿本文の断片どうしを区別します。テンプレートが、それが何で
// あるかを知るためにテキストをもう一度読むのではなく、そのものとして描画できるように
// するためです。
type PostBodyTokenKind int

const (
	// PostBodyText is a run of the body that stands for itself. It is the zero
	// value, so a body no rule matches anywhere is one piece of text.
	//
	// [Ja] PostBodyText は、それ自身を表す本文の断片です。ゼロ値であるため、どの規則
	// にも一致しない本文は 1 つのテキストになります。
	PostBodyText PostBodyTokenKind = iota

	// PostBodyPostReference is a >>N naming a post the same thread carries. A
	// >>N naming one it does not is text: the convention is only a link when
	// there is somewhere for it to lead.
	//
	// [Ja] PostBodyPostReference は、同じスレッドが持つ投稿を名指す >>N です。持たない
	// 投稿を名指す >>N はテキストです。この慣習がリンクになるのは、連れて行く先がある
	// ときだけです。
	PostBodyPostReference

	// PostBodyURL is a web address written in the body.
	//
	// [Ja] PostBodyURL は本文に書かれたウェブアドレスです。
	PostBodyURL
)

// PostBodyToken is one piece of a post body, and what the template renders it
// as. The pieces are handed over already told apart because deciding what a run
// of text is belongs to Go rather than to markup: a template that had to
// recognize a reference would be recognizing it a second time, by a rule kept
// somewhere the storing side cannot see.
//
// [Ja] PostBodyToken は投稿本文の断片 1 つと、テンプレートがそれを何として描画するかです。
// 区別の付いた状態で渡すのは、テキストの断片が何であるかを決めるのがマークアップではなく
// Go の仕事だからです。参照を認識せねばならないテンプレートは、それを 2 度目に、しかも
// 保存する側からは見えない場所に置かれた規則で認識することになります。
type PostBodyToken struct {
	Kind PostBodyTokenKind

	// Text is the run of the body this token stands for, as it was written.
	// Every kind carries it and every kind renders it, the two link kinds
	// included: what the author typed is the label, since nothing else about a
	// reply number or an address describes it better.
	//
	// [Ja] Text はこのトークンが表す本文の断片で、書かれたままの形です。どの種類も
	// これを運び、どの種類もこれを描画します。リンクになる 2 種類も同じで、書き手が
	// 打った文字がそのままラベルになります。レス番号やアドレスについて、それ以上に
	// うまく述べるものが他に無いためです。
	Text string

	// Number is the reply number a PostBodyPostReference names, and zero for
	// every other kind. It is what the link is built from, rather than the
	// digits in Text, so that >>007 leads where >>7 leads.
	//
	// [Ja] Number は PostBodyPostReference が名指すレス番号で、他の種類では 0 です。
	// リンクを組み立てるのは Text の中の数字ではなくこちらです。>>007 が >>7 と同じ
	// 場所へ繋がるようにするためです。
	Number int
}

// PostBody is a post's body as the template renders it: the body split into the
// runs of text, reply references, and addresses it is made of.
//
// The splitting happens here rather than in the template because the alternative
// is building HTML in a string and handing it over as raw markup, which puts the
// escaping of everything a visitor wrote into the hands of whoever next edits
// that string. Handing over pieces keeps templ escaping every one of them.
//
// [Ja] PostBody は、テンプレートが描画する形の投稿本文、すなわちテキストの断片・レス
// 参照・アドレスへと分解した本文です。
//
// 分解をテンプレートではなくここで行うのは、そうしない場合の道が、HTML を文字列として
// 組み立てて生のマークアップとして渡すことになるためです。それは訪問者が書いたもの
// すべてのエスケープを、次にその文字列を編集する人の手に委ねます。断片として渡せば、
// そのひとつひとつを templ がエスケープし続けます。
type PostBody struct {
	Tokens []PostBodyToken
}

// postBodyURLPattern matches a web address as it is written in a body: one of
// the two schemes a page can be fetched with, followed by the characters an
// address is written with.
//
// Only those two schemes are matched, so no other scheme reaches an href from a
// body, and they are matched however they are capitalized, since a scheme is
// case-insensitive. The characters after it are the ASCII ones a URL is made of
// (RFC 3986), which is what ends the address where the sentence around it
// resumes: Japanese runs on without spaces, so a body reading "…/help を見て"
// and one reading "…/helpを見て" would otherwise be told apart by a space the
// writer never thought about. It also leaves a >>N written straight after an
// address outside of it.
//
// The cost is an address written with the characters themselves rather than
// percent-encoded, whose non-ASCII part falls outside the link. That is the
// rarer of the two: an address is far more often pasted than typed, and a
// browser percent-encodes what it hands to the clipboard.
//
// [Ja] postBodyURLPattern は、本文に書かれたままのウェブアドレスに一致します。ページを
// 取得できる 2 つのスキームのいずれかと、それに続くアドレスを書くための文字です。
//
// 一致するのはその 2 つのスキームだけであるため、本文から href へ到達する他のスキームは
// ありません。スキームは大文字小文字を区別しないため、どちらの綴り方で書かれていても
// 一致させます。その後ろに続くのは URL を構成する ASCII 文字 (RFC 3986) であり、これが、
// 周囲の文が再開する場所でアドレスを終わらせます。日本語は空白を置かずに続くため、そう
// しなければ「…/help を見て」と書かれた本文と「…/helpを見て」と書かれた本文が、書き手が
// 意識してもいない空白によって区別されることになります。これはまた、アドレスの直後に
// 書かれた >>N をその外側に残します。
//
// 代償は、パーセントエンコードせずに文字そのもので書かれたアドレスの、ASCII でない部分が
// リンクから外れることです。2 つのうちではこちらが稀です。アドレスは打たれるよりはるかに
// 多く貼り付けられ、ブラウザがクリップボードへ渡すものはパーセントエンコード済みです。
var postBodyURLPattern = regexp.MustCompile(`(?i:https?)://[A-Za-z0-9\-._~:/?#\[\]@!$&'()*+,;=%]+`)

// postBodyURLTail holds the characters that close the sentence an address was
// written into rather than the address itself. An address written mid-sentence
// runs up against the punctuation that follows it with no space in between, so
// the match has to give those characters back to the text around it.
//
// Closing parentheses and square brackets are missing from this set on purpose:
// either can be part of an address or the end of the text around it, and which
// one it is can be told from whether the address opened a matching bracket of
// its own, which is what trimPostBodyURL checks.
//
// [Ja] postBodyURLTail は、アドレスそのものではなく、アドレスが書き込まれた文を閉じる
// 文字を持ちます。文の途中に書かれたアドレスは、間に空白を挟まずに後続の句読点と接する
// ため、一致した範囲はそれらの文字を周囲のテキストへ返さねばなりません。
//
// 丸括弧と角括弧の閉じ括弧が意図的にこの集合から外れているのは、どちらもアドレスの一部で
// あることも周囲のテキストの終わりであることもあるためです。どちらであるかは、アドレス
// 自身が対応する括弧を開いているかどうかで判別でき、それを見るのが trimPostBodyURL です。
const postBodyURLTail = `.,:;!?'`

// NewPostBody splits body into the pieces the template renders it as.
//
// postNumbers is the set of reply numbers the thread carries. A >>N it holds
// becomes a link to that post, and a >>N it does not stays text — a thread is
// where a reply number means anything, and the body alone cannot say which
// numbers are there.
//
// [Ja] NewPostBody は body を、テンプレートがそれを描画するための断片へ分解します。
//
// postNumbers はスレッドが持つレス番号の集合です。そこに含まれる >>N はその投稿への
// リンクになり、含まれない >>N はテキストのままになります。レス番号が意味を持つ場所は
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

// postBodyMark is one run of a body that renders as something other than plain
// text, and where in the body it starts. Its length is the length of the token's
// text, so a mark and the run it stands for cannot disagree.
//
// [Ja] postBodyMark は、ただのテキスト以外のものとして描画される本文の断片 1 つと、
// それが本文のどこから始まるかです。長さはトークンのテキストの長さそのものであるため、
// mark とそれが表す断片が食い違うことはありません。
type postBodyMark struct {
	start int
	token PostBodyToken
}

// postBodyMarks finds everything in body that renders as more than text, in the
// order it is written.
//
// The two scans cannot overlap: an address never contains a > (see
// postBodyURLPattern), and a reference always begins with one, so ordering the
// results by where they start is enough to lay them end to end.
//
// [Ja] postBodyMarks は、body の中でテキスト以上のものとして描画されるものを、書かれた
// 順にすべて見つけます。
//
// 2 つの走査が重なることはありません。アドレスが > を含むことはなく (postBodyURLPattern
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

		// Trimming can leave a bare scheme, which happens when what followed it
		// was the punctuation closing the sentence. That addresses nothing, so
		// it goes back to being the text it reads as.
		//
		// [Ja] 切り詰めた結果、スキームだけが残ることがあります。その後ろにあったものが
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

// trimPostBodyURL gives back the punctuation the sentence around an address put
// after it, leaving the address itself.
//
// A closing parenthesis or square bracket is only the surrounding text's when
// the address has no matching opening one of its own left unclosed, so that an
// address whose brackets balance keeps them while one written inside brackets
// does not take the outer one with it.
//
// [Ja] trimPostBodyURL は、アドレスの周りの文がその後ろに置いた句読点を返し、アドレス
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
