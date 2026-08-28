package seed

import (
	"context"
	"database/sql"
	"fmt"
)

// cleanupTables lists the tables a run rebuilds from scratch, in the order they
// are emptied. Every run starts from an empty set of them, so that what is on
// screen is always what the current code generates.
//
// The order is children before parents. SQLite enforces the foreign keys (the
// connection sets foreign_keys=on), and a DELETE that leaves a row pointing at
// a row that is gone fails: boards before categories, because a category in use
// is RESTRICT, and posts before threads, because a thread points back at its
// last post. The cascades would settle most of these on their own, but relying
// on them would make the order say less than it does.
//
// communities is placed by what it holds rather than by a constraint: nothing
// points at that row (a category belongs to the instance as a whole), so it is
// emptied after the content the community is made of.
//
// [Ja] cleanupTables は、実行が毎回作り直すテーブルを、空にする順に並べたものです。
// 実行のたびにこれらを空にしてから始めることで、画面に出るデータが常に現在のコードの
// 生成結果と一致するようにします。
//
// 順序は子から親です。SQLite は外部キーを強制しており (接続が foreign_keys=on を設定
// します)、消えた行を指したままにする DELETE は失敗します。掲示板がカテゴリーより先なのは
// 使用中のカテゴリーが RESTRICT であるためで、投稿がスレッドより先なのはスレッドが最終
// 投稿を指し返しているためです。多くは CASCADE が自ずと解決しますが、それに頼ると順序が
// 語る内容が減ります。
//
// communities の位置を決めているのは制約ではなく、それが抱えるものです。この行を指す
// ものは無く (カテゴリーはインスタンス全体に属します)、そのためコミュニティを構成する
// 中身を空にした後に空にしています。
var cleanupTables = []string{
	"post_references",
	"posts",
	"threads",
	"boards",
	"categories",
	"communities",
	"user_roles",
	"user_two_factor_auths",
	"password_reset_tokens",
	"email_confirmations",
	"user_sessions",
	"user_passwords",
	"users",
}

// preservedTables lists the tables the cleanup leaves alone: the bookkeeping the
// database needs to stay usable (the migration versions, the job queue's own
// state) and the roles the community defines.
//
// The roles cannot be made again once they are gone: nothing in the application
// creates them, so a run that emptied them would take away rows only a
// hand-written statement could put back. The assignments in user_roles are
// emptied with the users they belong to, which leaves the definitions standing
// without anything pointing at them.
//
// The list is exhaustive on purpose: a test compares it together with
// cleanupTables against the tables the schema actually has, so a table added
// later has to be placed on one side or the other instead of being silently left
// behind by the cleanup.
//
// [Ja] preservedTables はクリーンアップが触らないテーブルの一覧です。データベースを
// 使い続けるための管理情報 (マイグレーションのバージョン、ジョブキュー自身の状態) と、
// コミュニティが定義するロールです。
//
// ロールは消すと作り直せません。アプリケーションにこれを作るものが無いため、これを
// 空にする実行は、手で書いた文でしか戻せない行を奪うことになります。user_roles の
// 割り当ては、それが属するユーザーとともに空になるため、定義だけが、それを指すものの
// ない状態で残ります。
//
// この一覧を網羅的にしているのは意図的で、cleanupTables と合わせてスキーマの実際の
// テーブルと突き合わせるテストがあります。後から追加されたテーブルは、クリーンアップから
// 黙って漏れるのではなく、どちらかへ必ず振り分けることになります。
var preservedTables = []string{
	"goose_db_version",
	"river_job",
	"river_leader",
	"river_migration",
	"river_notification",
	"river_queue",
	"roles",
}

// cleanup empties every table in cleanupTables.
//
// The rows go one table at a time rather than in one statement: SQLite has no
// TRUNCATE, and a DELETE names a single table. The whole run shares one
// transaction, so a failure partway leaves the database as it was rather than
// half emptied.
//
// [Ja] cleanup は cleanupTables のテーブルをすべて空にします。
//
// 1 文にまとめず 1 テーブルずつ消すのは、SQLite に TRUNCATE が無く、DELETE が名指し
// できるテーブルが 1 つだからです。実行全体が 1 つのトランザクションを共有するため、
// 途中で失敗しても、データベースは半分空になった状態ではなく元のまま残ります。
func cleanup(ctx context.Context, tx *sql.Tx) error {
	for _, table := range cleanupTables {
		// The table name is interpolated because a placeholder cannot stand for
		// an identifier. The names come from the fixed package-level list above
		// and never from input, so gosec's warning about a built statement does
		// not apply here.
		//
		// [Ja] テーブル名を埋め込むのは、プレースホルダーが識別子の代わりになれない
		// ためです。名前はいずれも上の固定のパッケージレベルの一覧に由来し、入力から
		// 来ることはないため、組み立てた文に対する gosec の指摘はここでは当たりません。
		//nolint:gosec // G202
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("failed to empty the table %s: %w", table, err)
		}
	}

	return nil
}
