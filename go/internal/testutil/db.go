// testutilパッケージは、テスト用の共有ヘルパー (テストごとのSQLiteデータベースと、
// そこへ行を投入するビルダーなど) を提供します。
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/groobb/groobb/go/internal/database"
)

// databaseFileNameはすべてのテスト用データベースファイルに付ける名前です。
// どのファイルもそれぞれ専用のディレクトリに置かれるため、名前に求められるのは
// スタックトレースや残った一時ディレクトリの中で見分けが付くことだけです。
const databaseFileName = "groobb.sqlite"

// schemaSnapshotは、マイグレーションをすべて適用した空のデータベースファイルを
// 保持します。テストバイナリごとに1度だけ構築し、データベースを要求するテストごとに
// 複製します。
var (
	schemaSnapshot     []byte
	schemaSnapshotErr  error
	schemaSnapshotOnce sync.Once
)

// SetupDBはテストに専用のマイグレーション済みSQLiteデータベースを与え、その
// 接続プールを返します。プールはテストの終了時にクローズします。
//
// 1つのデータベースを共有してテストごとにトランザクションをロールバックするのではなく、
// テストごとにファイルを分けるのは、SQLiteがファイル全体で同時に1つのライターしか
// 許さないためです。並行するテストがそれぞれ書き込みトランザクションを保持すると、その
// 1つのライター上で直列化されてしまいます。インメモリではなくファイルにするのは、本番が
// 動作するのと同じWALモードとロック競合をテストでも通るようにするためです。
func SetupDB(t *testing.T) *database.DB {
	t.Helper()

	path := SetupDBPath(t)

	db, err := database.Open(context.Background(), path)
	if err != nil {
		t.Fatalf("テスト用データベースのオープンに失敗: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("テスト用データベースのクローズに失敗: %v", err)
		}
	})

	return db
}

// SetupDBPathはテストに専用のマイグレーション済みSQLiteデータベースファイルを
// 与え、そのパスを返します。SetupDBが返すプールを受け取るのではなく、ファイルを自分で
// 開く呼び出し元 (自身の接続を所有するワーカークライアント) のためのものです。
func SetupDBPath(t *testing.T) string {
	t.Helper()

	snapshot, err := prepareSchemaSnapshot()
	if err != nil {
		t.Fatalf("テスト用データベースのスキーマの準備に失敗: %v", err)
	}

	// t.TempDirはテストごとに、そのテストの終了時に削除されるディレクトリを
	// 与える。データベースファイルもそれと共に消える。ディレクトリの削除は呼び出し元が
	// 登録するcleanupより先に登録されるため、実行は呼び出し元が開いたものを閉じた後になる。
	path := filepath.Join(t.TempDir(), databaseFileName)
	if err := os.WriteFile(path, snapshot, 0o600); err != nil {
		t.Fatalf("テスト用データベースファイルの書き出しに失敗: %v", err)
	}

	return path
}

// prepareSchemaSnapshotは、すべてのテスト用データベースの複製元となる
// スナップショットを返します。最初の呼び出しで構築します。
//
// 最初の呼び出しではbcryptのコストも下げます。これはパッケージレベルの変数であり、
// そうしなければ並行するテストが競合して書き込むことになります。どちらも同じ1度きりの
// セットアップに属します。SetupDBを呼ぶテストはデータベースを相手にするテストであり、
// 本番のコストでパスワードをハッシュ化するとそのテストの実行時間の大半を占めるためです。
func prepareSchemaSnapshot() ([]byte, error) {
	schemaSnapshotOnce.Do(func() {
		LowerBcryptCost()

		schemaSnapshot, schemaSnapshotErr = buildSchemaSnapshot()
	})

	return schemaSnapshot, schemaSnapshotErr
}

// buildSchemaSnapshotは使い捨てのデータベースをマイグレートし、その結果の
// ファイルを返します。
//
// 各テストがスキーマを得る手段を、データベースを直接マイグレートすることではなく、この
// スナップショットの複製にしているのは、ファイルの書き出しにかかるコストが
// マイグレーションの適用の約5分の1であり、しかもgooseが毎回すべてのマイグレーションを
// 読んで適用する以上、その差はマイグレーションが増えるほど開いていくためです。
//
// スナップショットをディスク上のファイルではなくメモリに持つのは、ファイルにすると
// それを複製するすべてのテストより長く生存させる必要があり、テストバイナリには後から
// それを削除できる地点が無いためです。
func buildSchemaSnapshot() ([]byte, error) {
	ctx := context.Background()

	dir, err := os.MkdirTemp("", "groobb-schema-snapshot-")
	if err != nil {
		return nil, fmt.Errorf("スナップショットのディレクトリの作成に失敗: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, databaseFileName)

	db, err := database.Open(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("スナップショットのデータベースのオープンに失敗: %w", err)
	}

	if err := database.Migrate(ctx, db.Writer); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("スナップショットのデータベースのマイグレーションに失敗: %w", err)
	}

	// ファイルを読む前にクローズする。WALモードではマイグレーションは
	// write-ahead log上にあり、SQLiteが最後のコネクションのクローズでそれを
	// データベースファイルへ畳み込むため。それより早く読むと、複製しても空の
	// データベースにしかならないファイルが得られる。
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("スナップショットのデータベースのクローズに失敗: %w", err)
	}

	// pathは本関数が今作った一時ディレクトリと定数のファイル名を結合したもので、
	// 外部から渡されることはないため、gosec G304 (変数のパスからのファイル読み込みの
	// 指摘) はここではfalse positiveである。
	//nolint:gosec // G304
	snapshot, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("スナップショットのデータベースの読み込みに失敗: %w", err)
	}

	return snapshot, nil
}
