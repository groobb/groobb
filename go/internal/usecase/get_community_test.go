package usecase_test

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
)

// newGetCommunityUsecase builds the UseCase over a database the test owns,
// returning that database alongside it so the test can decide whether the
// instance has been set up before reading it back.
//
// [Ja] newGetCommunityUsecase はテストが所有するデータベース上に UseCase を構築し、
// 読み戻す前にインスタンスが立ち上げ済みかどうかをテストが決められるよう、その
// データベースも併せて返します。
func newGetCommunityUsecase(t *testing.T) (*usecase.GetCommunityUsecase, *database.DB) {
	t.Helper()

	db := testutil.SetupDB(t)

	return usecase.NewGetCommunityUsecase(repository.NewCommunityRepository(db)), db
}

// TestGetCommunityUsecase_Execute verifies that Execute returns the community
// this instance hosts.
//
// [Ja] TestGetCommunityUsecase_Execute は、Execute がこのインスタンスが運営する
// コミュニティを返すことを検証します。
func TestGetCommunityUsecase_Execute(t *testing.T) {
	t.Parallel()

	uc, db := newGetCommunityUsecase(t)
	ctx := context.Background()

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", "ジャズ喫茶"); err != nil {
		t.Fatalf("communities への INSERT に失敗: %v", err)
	}

	output, err := uc.Execute(ctx)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Community == nil {
		t.Fatalf("output.Community = nil, want a community")
	}
	if output.Community.Name != "ジャズ喫茶" {
		t.Errorf("output.Community.Name = %q, want %q", output.Community.Name, "ジャズ喫茶")
	}
}

// TestGetCommunityUsecase_Execute_NotSetUp verifies that an instance whose
// community has not been created yet is answered with a nil community and no
// error, which is what lets the caller render the pages without the name instead
// of failing the request.
//
// [Ja] TestGetCommunityUsecase_Execute_NotSetUp は、コミュニティがまだ作られていない
// インスタンスに対し、エラーではなく nil のコミュニティで応答することを検証します。
// これにより呼び出し側は、リクエストを失敗させず名前の無いままページを描画できます。
func TestGetCommunityUsecase_Execute_NotSetUp(t *testing.T) {
	t.Parallel()

	uc, _ := newGetCommunityUsecase(t)

	output, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if output.Community != nil {
		t.Errorf("output.Community = %+v, want nil", output.Community)
	}
}
