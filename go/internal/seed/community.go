package seed

import (
	"context"
	"database/sql"
	"fmt"
)

// matureCommunityNameとcoldStartCommunityNameは、各プロファイルのコミュニティが
// 自身を何と呼ぶかです。2つは形を揃え、頭の語だけを変えています。サイドバーの見出しと
// 各ページのタイトルの接尾辞が、開発者の目の前のデータベースがどちらの状態を持つのかを
// 述べるようにするためです。1つの名前を共有すると、この行が読まれる唯一の場所である
// 画面上で、2つの実行を見分けられなくなります。
const (
	matureCommunityName    = "ひだまり広場"
	coldStartCommunityName = "はじまりの広場"
)

// createCommunityStatementは、このインスタンスがどのコミュニティを運営するのかを
// 述べる唯一の行を書き込みます。
//
// 行をCommunityRepositoryではなくシード自身の文で入れています。同リポジトリはこの行を
// 読みますが作りません。アプリケーションにはまだコミュニティを作るものが無く、このために
// 足すCreateはシードだけが呼ぶInfrastructureになるためです。インスタンスの立ち上げが
// 画面を持つときは、その画面が必要とする書き込みをそのときに書きます。今日ただ1つある
// 呼び出し側の都合で、先回りして形を決めることはしません。
//
// idはSQLiteに委ねず書き下しています。テーブルが持ちうるのはid 1の行だけであり
// (CHECK (id = 1))、コミュニティを読むクエリはそのidで引きます。ここで名指しすることが、
// 実行が作る行を、アプリケーションが読む行にします。
const createCommunityStatement = "INSERT INTO communities (id, name) VALUES (1, ?)"

// generateCommunityは、生成する中身が属するコミュニティを作成します。
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
