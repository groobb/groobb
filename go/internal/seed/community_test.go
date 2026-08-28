package seed

import (
	"context"
	"testing"

	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// TestRunner_GenerateCommunity verifies that a run creates the row an instance
// reads its name from, under the name of the profile that was asked for and as
// the row the application looks the community up by. A run that wrote a
// different id would leave a database whose community is invisible to every
// screen, since the query that reads it selects the row id 1.
//
// [Ja] TestRunner_GenerateCommunity は、実行が、インスタンスが自身の名前を読み取る行を、
// 指定されたプロファイルの名前で、かつアプリケーションがコミュニティを引くときの行として
// 作成することを検証します。別の id で書いた実行は、どの画面からもコミュニティが見えない
// データベースを残します。それを読むクエリが id 1 の行を引くためです。
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
				t.Fatalf("generateCommunity() error = %v", err)
			}
			if err := tx.Commit(); err != nil {
				t.Fatalf("failed to commit the transaction: %v", err)
			}

			community, err := repository.NewCommunityRepository(db).Find(ctx)
			if err != nil {
				t.Fatalf("Find() error = %v", err)
			}
			if community == nil {
				t.Fatal("no community was created")
			}
			if community.Name != profile.communityName {
				t.Errorf("community name = %q, want %q", community.Name, profile.communityName)
			}
			if community.ID != 1 {
				t.Errorf("community id = %d, want 1", community.ID)
			}
		})
	}
}
