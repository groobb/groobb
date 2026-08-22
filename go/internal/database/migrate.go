package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"

	"github.com/groobb/groobb/go/db"
)

// Migrate applies every migration that has not been applied yet, in version
// order, and records each one in the version table goose keeps. It is safe to
// call on an up-to-date database, where it does nothing.
//
// The write pool is passed explicitly because a migration is a write and SQLite
// admits only one writer: running it on the read pool would either fail on the
// read-only connection or, worse, escape the single-writer design.
//
// [Ja] Migrate は未適用のマイグレーションをバージョン順にすべて適用し、goose が持つ
// バージョン管理テーブルに 1 本ずつ記録します。適用済みのデータベースに対して呼んでも
// 安全で、その場合は何もしません。
//
// 書き込み用プールを明示的に受け取るのは、マイグレーションが書き込みであり、SQLite が
// ライターを 1 つしか許さないためです。読み取り用プールで実行すると、読み取り専用の
// コネクション上で失敗するか、より悪ければ single-writer 設計を迂回してしまいます。
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

// Rollback reverts the migration that was applied most recently. It reports an
// error when there is nothing left to revert.
//
// [Ja] Rollback は最後に適用されたマイグレーションを 1 本だけ取り消します。取り消せる
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

// newMigrationProvider builds a goose provider that reads the migrations
// embedded in the binary and applies them to writer.
//
// The provider is created per call rather than kept as package state: it holds
// the database handle it was built with, and migrating is rare enough that
// parsing the migration list again costs nothing worth saving.
//
// [Ja] newMigrationProvider は、バイナリに埋め込まれたマイグレーションを読み、それを
// writer へ適用する goose の provider を組み立てます。
//
// provider をパッケージの状態として保持せず呼び出しごとに作るのは、provider が構築時の
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
