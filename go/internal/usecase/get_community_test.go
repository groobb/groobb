package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCommunityUsecaseはテストが所有するデータベース上にUseCaseを構築し、
// 読み戻す前にインスタンスが立ち上げ済みかどうかをテストが決められるよう、その
// データベースも併せて返します。
func newGetCommunityUsecase(t *testing.T) (*usecase.GetCommunityUsecase, *database.DB) {
	t.Helper()

	db := testutil.SetupDB(t)

	return usecase.NewGetCommunityUsecase(repository.NewCommunityRepository(db)), db
}

// TestGetCommunityUsecase_Executeは、Executeがこのインスタンスが運営する
// コミュニティを返すことを検証します。
func TestGetCommunityUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, db := newGetCommunityUsecase(t)
	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if output.Community == nil {
		t.Fatalf("output.Community = nil、期待値 = 非nil")
	}
	if output.Community.Name != "ジャズ喫茶" {
		t.Errorf("output.Community.Name = %q、期待値 = %q", output.Community.Name, "ジャズ喫茶")
	}
}

// TestGetCommunityUsecase_Execute_NotSetUpは、コミュニティがまだ作られていない
// インスタンスに対し、エラーではなくnilのコミュニティで応答することを検証します。
// これにより呼び出し側は、リクエストを失敗させず名前の無いままページを描画できます。
func TestGetCommunityUsecase_Execute_NotSetUp(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCommunityUsecase(t)

	output, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute()のエラー = %v", err)
	}

	if output.Community != nil {
		t.Errorf("output.Community = %+v、期待値 = nil", output.Community)
	}
}
