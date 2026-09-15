package worker

import (
	"testing"

	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

// 本テストはRiverのスキーマが本プロジェクト自身のマイグレーションより
// 先行して乖離するのを防ぐ。rivermigrate.AllVersions() はリンク済みのRiver
// ライブラリに埋め込まれたマイグレーションバージョンを読むだけで (データベース
// I/Oなし)、その最大値はライブラリが認識する最新バージョンを表す。Riverの
// アップグレード (dependabotによるbumpなど) がappliedRiverMigrationVersionを
// 超えるバージョンを導入すると本テストは失敗し、Riverの新スキーマに追随する
// マイグレーションの追加が必要であることを知らせる。
func TestAppliedRiverMigrationVersionMatchesLibrary(t *testing.T) {
	t.Parallel()

	// ここではnilプールで安全: AllVersions() は埋め込みのマイグレーション
	// ファイルを読むだけで、データベースには一切アクセスしない。
	migrator, err := rivermigrate.New(riversqlite.New(nil), nil)
	if err != nil {
		t.Fatalf("rivermigrate.New() でエラー: %v", err)
	}

	versions := migrator.AllVersions()
	if len(versions) == 0 {
		t.Fatal("AllVersions() が空のスライスを返した")
	}

	// AllVersions() はバージョン昇順にソートされているため、末尾の要素が
	// リンク済みのRiverライブラリが認識する最新バージョン。
	latest := versions[len(versions)-1].Version

	if latest != appliedRiverMigrationVersion {
		t.Errorf(
			"Riverライブラリの最新マイグレーションversion (%d) が適用済みversion定数appliedRiverMigrationVersion (%d) と一致しません。"+
				"Riverが新しいマイグレーションを導入した可能性があります。"+
				"Riverのマイグレータで新バージョンのSQLを生成してマイグレーションを追加し、"+
				"appliedRiverMigrationVersionを %d に更新してください。",
			latest, appliedRiverMigrationVersion, latest,
		)
	}
}
