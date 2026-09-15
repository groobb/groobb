package usecase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// TestGetAdminHomeUsecase_Executeは、誰が管理ハブを許されるかを検証します。管理者と
// 運用者は許され、管理画面のどれかのスコープを持つロールの保持者も許され、ロールを1つも
// 持たない人はAppErrCodeForbiddenで拒まれます。
//
// より狭いロールを含めるのは、ハブがサイドバーの導線の行き先であるためです。1つの画面を
// 開いてよい操作者は、そこへ入る唯一の道で追い返されるのではなく、その画面を並べるページに
// 辿り着けなければなりません。
func TestGetAdminHomeUsecase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// seedは操作者のロールをデータベースへ置き、その操作者を返す。
		seed          func(t *testing.T, db *database.DB) usecase.Actor
		wantForbidden bool
	}{
		{
			name: "adminロールを持つ利用者は管理ハブを開ける",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				testutil.NewUserRoleBuilder(t, db).WithUserID(userID).Build()
				return usecase.UserActor(userID)
			},
		},
		{
			name: "利用者を読むスコープだけを持つ利用者も管理ハブを開ける",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				userID := testutil.NewUserBuilder(t, db).Build()
				testutil.NewRoleBuilder(t, db).
					WithName(model.RoleName("user_reader")).
					WithScopes([]model.Scope{model.ScopeUserRead}).
					Build()
				testutil.NewUserRoleBuilder(t, db).
					WithUserID(userID).
					WithRoleName(model.RoleName("user_reader")).
					Build()
				return usecase.UserActor(userID)
			},
		},
		{
			name: "運用者は管理ハブを開ける",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				return usecase.OperatorActor()
			},
		},
		{
			name: "ロールを持たない利用者は拒まれる",
			seed: func(t *testing.T, db *database.DB) usecase.Actor {
				t.Helper()
				return usecase.UserActor(testutil.NewUserBuilder(t, db).Build())
			},
			wantForbidden: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			db := testutil.SetupDB(t)
			uc := usecase.NewGetAdminHomeUsecase(repository.NewRoleRepository(db))
			ctx := i18n.SetLocale(context.Background(), model.LocaleJa)

			err := uc.Execute(ctx, usecase.GetAdminHomeInput{Actor: tt.seed(t, db)})

			if !tt.wantForbidden {
				if err != nil {
					t.Fatalf("Execute()のエラー = %v、期待値 = nil", err)
				}
				return
			}
			assertAppErrCode(t, err, model.AppErrCodeForbidden)
		})
	}
}

// TestGetAdminHomeUsecase_Execute_RoleLookupFailureは、操作者のロール取得失敗が
// 処理の文脈を保って伝搬し、AppErrCodeForbiddenに置き換わらないことを検証します。
// データベース障害は権限の有無を表さないためです。
func TestGetAdminHomeUsecase_Execute_RoleLookupFailure(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	userID := testutil.NewUserBuilder(t, db).Build()
	if err := db.Reader.Close(); err != nil {
		t.Fatalf("ReaderのClose()のエラー = %v", err)
	}
	uc := usecase.NewGetAdminHomeUsecase(repository.NewRoleRepository(db))

	err := uc.Execute(context.Background(), usecase.GetAdminHomeInput{
		Actor: usecase.UserActor(userID),
	})

	if err == nil {
		t.Fatal("Execute()のエラー = nil、エラーを期待")
	}
	if ae := model.AsAppError(err); ae != nil {
		t.Errorf("Execute()のエラー = %v、期待値 = AppErrorではないシステムエラー", err)
	}
	if !strings.Contains(err.Error(), "操作者のロールの取得に失敗") {
		t.Errorf("Execute()のエラー = %q、ロール取得の文脈を含むことを期待", err)
	}
}
