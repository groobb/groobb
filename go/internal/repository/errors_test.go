package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestIsUniqueViolation verifies that IsUniqueViolation answers true for both
// result codes SQLite uses to reject a duplicate key, and false for anything
// else.
//
// The two codes are covered separately because SQLite reports a rejected write
// against an INTEGER PRIMARY KEY (the rowid) as a primary-key violation and every
// other unique index as a unique violation. Only the latter arises on the
// application's own paths (see the email-change apply), so without this test the
// primary-key half of the predicate could be dropped without any test noticing.
//
// The negative cases pin down that the predicate answers for duplicate keys
// specifically rather than for a driver error or a failed write in general: a
// NOT NULL rejection is also a constraint failure from the same driver, and
// sql.ErrNoRows is the error the repositories already turn into an absent row.
//
// [Ja] TestIsUniqueViolation は、IsUniqueViolation が SQLite の「キーの重複」を表す 2 つの
// 結果コードのどちらにも true を返し、それ以外には false を返すことを検証します。
//
// 2 つのコードを別々に覆うのは、SQLite が INTEGER PRIMARY KEY (rowid) に対する拒否を主キー
// 違反、それ以外の一意インデックスに対する拒否を一意制約違反として報告するためです。
// アプリケーション自身の経路で起きるのは後者だけであり (メール変更の適用を参照)、この
// テストが無いと述語の主キー側は、どのテストにも気づかれずに落とせてしまいます。
//
// 異常系は、この述語が「ドライバのエラー全般」や「書き込みの失敗全般」ではなく、キーの重複に
// 対して答えるものであることを固定します。NOT NULL による拒否も同じドライバから来る制約違反
// であり、sql.ErrNoRows はリポジトリが既に「行が無い」に変換しているエラーです。
func TestIsUniqueViolation(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, db).WithEmail("taken@example.com").WithAtname("taken").Build()

	tests := []struct {
		name string
		err  func(t *testing.T) error
		want bool
	}{
		{
			name: "一意インデックスの違反",
			err: func(t *testing.T) error {
				t.Helper()

				_, err := db.Writer.ExecContext(ctx,
					`INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)`,
					"taken@example.com", "other", "ja", "Asia/Tokyo",
				)
				return err
			},
			want: true,
		},
		{
			name: "INTEGER PRIMARY KEY (rowid) の違反",
			err: func(t *testing.T) error {
				t.Helper()

				_, err := db.Writer.ExecContext(ctx,
					`INSERT INTO users (id, email, atname, locale, time_zone) VALUES (?, ?, ?, ?, ?)`,
					int64(userID), "free@example.com", "free", "ja", "Asia/Tokyo",
				)
				return err
			},
			want: true,
		},
		{
			name: "別種の制約違反 (NOT NULL)",
			err: func(t *testing.T) error {
				t.Helper()

				_, err := db.Writer.ExecContext(ctx,
					`INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)`,
					nil, "notnull", "ja", "Asia/Tokyo",
				)
				return err
			},
			want: false,
		},
		{
			name: "ドライバ由来でないエラー",
			err:  func(*testing.T) error { return sql.ErrNoRows },
			want: false,
		},
		{
			name: "エラーが無い",
			err:  func(*testing.T) error { return nil },
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.err(t)
			if tt.want && err == nil {
				t.Fatal("書き込みが失敗する想定だが nil が返った")
			}
			if got := repository.IsUniqueViolation(err); got != tt.want {
				t.Errorf("IsUniqueViolation(%v) = %v, want %v", err, got, tt.want)
			}
		})
	}

	// The predicate has to see through a wrapped error: a repository returns the
	// driver error as-is, and the UseCase that receives it adds context with %w
	// before the decision to treat the race as a validation failure is made.
	//
	// [Ja] 述語はラップされたエラーも見通す必要がある。リポジトリはドライバのエラーを
	// そのまま返し、それを受け取る UseCase が、競合をバリデーション失敗として扱うか判断する
	// 前に %w で文脈を足すため。
	t.Run("ラップされた一意インデックスの違反", func(t *testing.T) {
		_, err := db.Writer.ExecContext(ctx,
			`INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)`,
			"taken@example.com", "wrapped", "ja", "Asia/Tokyo",
		)
		if err == nil {
			t.Fatal("書き込みが失敗する想定だが nil が返った")
		}
		if !repository.IsUniqueViolation(fmt.Errorf("ユーザーの作成に失敗: %w", err)) {
			t.Errorf("IsUniqueViolation() = false, want true (%%w で包んだ一意制約違反): %v", err)
		}
	})
}
