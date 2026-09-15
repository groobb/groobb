// dbパッケージは、サーバーバイナリに同梱するSQLを埋め込みます。
package db

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrationsはマイグレーションファイルを、それらを収めたディレクトリを根とする
// ファイルシステムとして返します。ディスクから読むのではなく埋め込むのは、セルフホスト
// されたインスタンスが、どのディレクトリから実行してもバイナリだけでデータベースを
// マイグレートできるようにするためです。
func Migrations() (fs.FS, error) {
	return fs.Sub(migrationsFS, "migrations")
}
