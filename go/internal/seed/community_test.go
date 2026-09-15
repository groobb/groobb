package seed

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestRunner_GenerateCommunityは、実行が、インスタンスが自身の名前を読み取る行を、
// 指定されたプロファイルの名前で、かつアプリケーションがコミュニティを引くときの行として
// 作成することを検証します。別のidで書いた実行は、どの画面からもコミュニティが見えない
// データベースを残します。それを読むクエリがid 1の行を引くためです。
func TestRunner_GenerateCommunity(t *testing.T) {
	t.Parallel()

	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			db := testutil.SetupDB(t)

			runner := newTestRunner(db)
			runner.profile = profile

			tx := beginTx(t, db)
			if err := runner.generateCommunity(ctx, tx, &state{}); err != nil {
				t.Fatalf("generateCommunity()のエラー = %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("トランザクションのコミットに失敗: %v", err)
			}

			community, err := repository.NewCommunityRepository(db).Find(ctx)
			if err != nil {
				t.Fatalf("Find()のエラー = %v", err)
			}
			if community == nil {
				t.Fatal("コミュニティが作成されていない")
			}
			if community.Name != profile.communityName {
				t.Errorf("コミュニティ名 = %q、期待値 = %q", community.Name, profile.communityName)
			}
			if community.ID != 1 {
				t.Errorf("コミュニティのID = %d、期待値 = 1", community.ID)
			}
		})
	}
}
