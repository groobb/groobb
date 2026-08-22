// Package db embeds the SQL that ships with the server binary.
//
// [Ja] db パッケージは、サーバーバイナリに同梱する SQL を埋め込みます。
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations returns the migration files, rooted at the directory that holds
// them. They are embedded rather than read from disk so that a self-hosted
// instance can migrate its database with nothing but the binary, wherever it is
// run from.
//
// [Ja] Migrations はマイグレーションファイルを、それらを収めたディレクトリを根とする
// ファイルシステムとして返します。ディスクから読むのではなく埋め込むのは、セルフホスト
// されたインスタンスが、どのディレクトリから実行してもバイナリだけでデータベースを
// マイグレートできるようにするためです。
func Migrations() (fs.FS, error) {
	return fs.Sub(migrationsFS, "migrations")
}
