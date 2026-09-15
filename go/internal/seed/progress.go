package seed

import (
	"fmt"
	"io"
)

// progressは生成ステップの進み具合を表示します。復帰文字で1行を書き換えるため、
// 数百行を作るステップでも表示は1行に収まり、ターミナルが流れません。
//
// カウンタをslogではなく素のio.Writerへ書くのは、これがコマンドを見ている開発者向けの
// 使い捨てのフィードバックであり、実行内容の記録ではないためです。実行内容はステップごとに
// 1回ログへ残します。
//
// ただしoutにはslogの書き込み先と同じストリームを渡す前提とします。カウンタとログ行が
// 書いた順に並ぶようにするためです。
type progress struct {
	out     io.Writer
	label   string
	total   int
	current int
}

// newProgressはlabelを見出しにしてtotalまでを数えるカウンタを開始します。
func newProgress(out io.Writer, label string, total int) *progress {
	p := &progress{out: out, label: label, total: total}
	p.render()

	return p
}

// advanceは項目を1件作成したことを記録します。
func (p *progress) advance() {
	p.current++
	p.render()
}

// finishは行を閉じ、次の出力が行頭から始まるようにします。
func (p *progress) finish() {
	p.write("\n")
}

// renderはカウンタをその場で描き直します。
func (p *progress) render() {
	p.write(fmt.Sprintf("\r  %s %d/%d", p.label, p.current, p.total))
}

// writeはsを出力先へ書き、書き込みエラーは意図的に捨てます。出力先が運ぶのは
// 進捗だけであり、データを投入できた実行がターミナルへ書けなかったことを理由に失敗しては
// ならないためです。
func (p *progress) write(s string) {
	_, _ = io.WriteString(p.out, s)
}
