package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/sqlitetime"
)

// BackdatePost moves a post to the moment a test places it at. The column's
// default is the moment the row is written, and the posts a test writes in a row
// land in the same millisecond, which says neither which of them is the latest
// nor how long ago its author last wrote.
//
// The update has to reach exactly one row. A post that is not there leaves the
// time the test was arranging as it was, and what the test asserts afterwards
// would be read against a state nobody set up.
//
// [Ja] BackdatePost は投稿を、テストがそれを置く時点へ移します。列の既定値は行を書いた
// 瞬間であり、テストが続けて書いた投稿は同じミリ秒に収まるため、どれが最新かも、その
// 作者が最後に書いたのがどれだけ前かも語りません。
//
// 更新はちょうど 1 行に届かなければなりません。存在しない投稿を指した更新は、テストが
// 整えようとしていた時刻をそのまま残し、その後にテストが検証するものは、誰も用意して
// いない状態に対して読まれることになります。
func BackdatePost(t *testing.T, db *database.DB, id model.PostID, createdAt time.Time) {
	t.Helper()

	result, err := db.Writer.ExecContext(
		context.Background(),
		"UPDATE posts SET created_at = ? WHERE id = ?",
		sqlitetime.Time(createdAt), int64(id),
	)
	if err != nil {
		t.Fatalf("テスト用投稿の作成時刻の変更に失敗: %v", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("テスト用投稿の作成時刻を変更した行数の取得に失敗: %v", err)
	}
	if affected != 1 {
		t.Fatalf("作成時刻を変更した投稿 = %d 件, want 1 件 (id=%v)", affected, id)
	}
}
