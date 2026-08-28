package seed

import (
	"context"
	"database/sql"
	"fmt"
)

// matureCommunityName and coldStartCommunityName are what the community of each
// profile calls itself. The two names share a shape and differ in their first
// word, so that the sidebar heading and the suffix of every page title say which
// state the database in front of the developer holds. A single shared name would
// leave the two runs indistinguishable on screen, at the one place this row is
// read.
//
// [Ja] matureCommunityName と coldStartCommunityName は、各プロファイルのコミュニティが
// 自身を何と呼ぶかです。2 つは形を揃え、頭の語だけを変えています。サイドバーの見出しと
// 各ページのタイトルの接尾辞が、開発者の目の前のデータベースがどちらの状態を持つのかを
// 述べるようにするためです。1 つの名前を共有すると、この行が読まれる唯一の場所である
// 画面上で、2 つの実行を見分けられなくなります。
const (
	matureCommunityName    = "ひだまり広場"
	coldStartCommunityName = "はじまりの広場"
)

// createCommunityStatement writes the single row that says which community this
// instance hosts.
//
// The row goes in through a statement of the seed's own rather than through
// CommunityRepository, which reads the row but does not create it: nothing in
// the application creates a community yet, so a Create added for this would be
// Infrastructure the seed alone calls. When setting an instance up grows a
// screen, the write it needs is written for that screen, rather than being
// shaped in advance by the one caller there is today.
//
// The id is written out instead of being left to SQLite. The table holds at most
// the row id 1 (CHECK (id = 1)), and the query that reads the community selects
// that id, so naming it here is what makes the row a run creates the row the
// application reads.
//
// [Ja] createCommunityStatement は、このインスタンスがどのコミュニティを運営するのかを
// 述べる唯一の行を書き込みます。
//
// 行を CommunityRepository ではなくシード自身の文で入れています。同リポジトリはこの行を
// 読みますが作りません。アプリケーションにはまだコミュニティを作るものが無く、このために
// 足す Create はシードだけが呼ぶ Infrastructure になるためです。インスタンスの立ち上げが
// 画面を持つときは、その画面が必要とする書き込みをそのときに書きます。今日ただ 1 つある
// 呼び出し側の都合で、先回りして形を決めることはしません。
//
// id は SQLite に委ねず書き下しています。テーブルが持ちうるのは id 1 の行だけであり
// (CHECK (id = 1))、コミュニティを読むクエリはその id で引きます。ここで名指しすることが、
// 実行が作る行を、アプリケーションが読む行にします。
const createCommunityStatement = "INSERT INTO communities (id, name) VALUES (1, ?)"

// generateCommunity creates the community the generated content belongs to.
//
// It runs before the other generators so that the phases and their progress are
// presented from the instance's community identity to the accounts and content
// within it. No later generator depends on this row: the order communicates the
// conceptual hierarchy rather than satisfying a database constraint.
//
// [Ja] generateCommunity は、生成する中身が属するコミュニティを作成します。
//
// 他の生成器より先に走ることで、フェーズとその進捗を、インスタンスのコミュニティの識別から、
// その中のアカウントとコンテンツへという順に示します。後続の生成器はこの行に依存せず、
// 順序が表すのはデータベースの制約ではなく概念上の階層です。
func (r *Runner) generateCommunity(ctx context.Context, tx *sql.Tx, _ *state) error {
	bar := newProgress(r.out, "community", 1)
	defer bar.finish()

	if _, err := tx.ExecContext(ctx, createCommunityStatement, r.profile.communityName); err != nil {
		return fmt.Errorf("failed to create the community %q: %w", r.profile.communityName, err)
	}

	bar.advance()

	return nil
}
