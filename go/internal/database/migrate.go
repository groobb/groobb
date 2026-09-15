package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"

	"github.com/groobb/groobb/go/db"
)

// Migrateは未適用のマイグレーションをバージョン順にすべて適用し、gooseが持つ
// バージョン管理テーブルに1本ずつ記録します。適用済みのデータベースに対して呼んでも
// 安全で、その場合は何もしません。
//
// 書き込み用プールを明示的に受け取るのは、マイグレーションが書き込みであり、SQLiteが
// ライターを1つしか許さないためです。読み取り用プールで実行すると、読み取り専用の
// コネクション上で失敗するか、より悪ければsingle-writer設計を迂回してしまいます。
func Migrate(ctx context.Context, writer *sql.DB) error {
	provider, err := newMigrationProvider(writer)
	if err != nil {
		return err
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("failed to apply the migrations: %w", err)
	}

	for _, result := range results {
		slog.InfoContext(ctx, "applied a migration", "version", result.Source.Version, "path", result.Source.Path)
	}

	return nil
}

// Rollbackは最後に適用されたマイグレーションを1本だけ取り消します。取り消せる
// ものが残っていない場合はエラーを返します。
func Rollback(ctx context.Context, writer *sql.DB) error {
	provider, err := newMigrationProvider(writer)
	if err != nil {
		return err
	}

	result, err := provider.Down(ctx)
	if err != nil {
		return fmt.Errorf("failed to roll back the migration: %w", err)
	}

	slog.InfoContext(ctx, "rolled back a migration", "version", result.Source.Version, "path", result.Source.Path)

	return nil
}

// newMigrationProviderは、バイナリに埋め込まれたマイグレーションを読み、それを
// writerへ適用するgooseのproviderを組み立てます。
//
// providerをパッケージの状態として保持せず呼び出しごとに作るのは、providerが構築時の
// データベースハンドルを保持することと、マイグレーションの実行頻度からしてマイグレーション
// 一覧を読み直す程度のコストは節約する価値が無いためです。
func newMigrationProvider(writer *sql.DB) (*goose.Provider, error) {
	migrations, err := db.Migrations()
	if err != nil {
		return nil, fmt.Errorf("failed to open the embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, writer, migrations)
	if err != nil {
		return nil, fmt.Errorf("failed to build the migration provider: %w", err)
	}

	return provider, nil
}
