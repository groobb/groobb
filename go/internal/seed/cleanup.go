package seed

import (
	"context"
	"database/sql"
	"fmt"
)

// cleanupTablesは、実行が毎回作り直すテーブルを、空にする順に並べたものです。
// 実行のたびにこれらを空にしてから始めることで、画面に出るデータが常に現在のコードの
// 生成結果と一致するようにします。
//
// 順序は子から親です。SQLiteは外部キーを強制しており (接続がforeign_keys=onを設定
// します)、消えた行を指したままにするDELETEは失敗します。掲示板がカテゴリーより先なのは
// 使用中のカテゴリーがRESTRICTであるためで、投稿がスレッドより先なのはスレッドが最終
// 投稿を指し返しているためです。多くはCASCADEが自ずと解決しますが、それに頼ると順序が
// 語る内容が減ります。
//
// communitiesの位置を決めているのは制約ではなく、それが抱えるものです。この行を指す
// ものは無く (カテゴリーはインスタンス全体に属します)、そのためコミュニティを構成する
// 中身を空にした後に空にしています。
//
// moderation_logsを先頭に置く理由は子テーブルと同じです。参照はON DELETE SET NULLであり
// 削除を拒みはしませんが、対象より後に空にすれば、列が既にNULLになった記録を空にすることに
// なり、前回の実行が何を記録したかを何も述べないものになります。
var cleanupTables = []string{
	"moderation_logs",
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

// preservedTablesはクリーンアップが触らないテーブルの一覧です。データベースを
// 使い続けるための管理情報 (マイグレーションのバージョン、ジョブキュー自身の状態) と、
// コミュニティが定義するロールです。
//
// ロールは消すと作り直せません。アプリケーションにこれを作るものが無いため、これを
// 空にする実行は、手で書いた文でしか戻せない行を奪うことになります。user_rolesの
// 割り当ては、それが属するユーザーとともに空になるため、定義だけが、それを指すものの
// ない状態で残ります。
//
// この一覧を網羅的にしているのは意図的で、cleanupTablesと合わせてスキーマの実際の
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

// cleanupはcleanupTablesのテーブルをすべて空にします。
//
// 1文にまとめず1テーブルずつ消すのは、SQLiteにTRUNCATEが無く、DELETEが名指し
// できるテーブルが1つだからです。実行全体が1つのトランザクションを共有するため、
// 途中で失敗しても、データベースは半分空になった状態ではなく元のまま残ります。
func cleanup(ctx context.Context, tx *sql.Tx) error {
	for _, table := range cleanupTables {
		// テーブル名を埋め込むのは、プレースホルダーが識別子の代わりになれない
		// ためです。名前はいずれも上の固定のパッケージレベルの一覧に由来し、入力から
		// 来ることはないため、組み立てた文に対するgosecの指摘はここでは当たりません。
		//nolint:gosec // G202
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("failed to empty the table %s: %w", table, err)
		}
	}

	return nil
}
