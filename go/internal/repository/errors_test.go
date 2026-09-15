package repository_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestIsUniqueViolationは、IsUniqueViolationがSQLiteの「キーの重複」を表す2つの
// 結果コードのどちらにもtrueを返し、それ以外にはfalseを返すことを検証します。
//
// 2つのコードを別々に覆うのは、SQLiteがINTEGER PRIMARY KEY (rowid) に対する拒否を主キー
// 違反、それ以外の一意インデックスに対する拒否を一意制約違反として報告するためです。
// アプリケーション自身の経路で起きるのは後者だけであり (メール変更の適用を参照)、この
// テストが無いと述語の主キー側は、どのテストにも気づかれずに落とせてしまいます。
//
// 異常系は、この述語が「ドライバのエラー全般」や「書き込みの失敗全般」ではなく、キーの重複に
// 対して答えるものであることを固定します。NOT NULLによる拒否も同じドライバから来る制約違反
// であり、sql.ErrNoRowsはリポジトリが既に「行が無い」に変換しているエラーです。
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
				t.Fatal("書き込みが失敗する想定だがnilが返った")
			}
			if got := repository.IsUniqueViolation(err); got != tt.want {
				t.Errorf("IsUniqueViolation(%v) = %v、期待値 = %v", err, got, tt.want)
			}
		})
	}

	// 述語はラップされたエラーも見通す必要がある。リポジトリはドライバのエラーを
	// そのまま返し、それを受け取るUseCaseが、競合をバリデーション失敗として扱うか判断する
	// 前に %wで文脈を足すため。
	t.Run("ラップされた一意インデックスの違反", func(t *testing.T) {
		_, err := db.Writer.ExecContext(ctx,
			`INSERT INTO users (email, atname, locale, time_zone) VALUES (?, ?, ?, ?)`,
			"taken@example.com", "wrapped", "ja", "Asia/Tokyo",
		)
		if err == nil {
			t.Fatal("書き込みが失敗する想定だがnilが返った")
		}
		if !repository.IsUniqueViolation(fmt.Errorf("ユーザーの作成に失敗: %w", err)) {
			t.Errorf("IsUniqueViolation() = false、期待値 = true (%%w で包んだ一意制約違反): %v", err)
		}
	})
}
