// Package static embeds the built front-end assets that ship inside the server
// binary.
//
// [Ja] static パッケージは、サーバーバイナリに同梱するビルド済みフロントエンド
// アセットを埋め込みます。
package static

import (
	"embed"
	"io/fs"
)

// The `all:` prefix keeps the patterns matching before `pnpm build` has written
// style.css and main.js. Without a build the two directories hold nothing but
// their .gitkeep placeholders, and go:embed rejects a directory that yields no
// embeddable file, which would break `go build` and `go test` on a fresh clone.
//
// [Ja] `all:` 接頭辞は、`pnpm build` が style.css と main.js を書き出す前でもパターンが
// 一致するようにするためのものです。ビルド前の 2 つのディレクトリには .gitkeep
// プレースホルダーしか無く、go:embed は埋め込めるファイルを 1 つも得られない
// ディレクトリを拒否するため、クローン直後の `go build` や `go test` が壊れます。
//
//go:embed all:css all:js
var assetsFS embed.FS

// Assets returns the built CSS and JavaScript, rooted at the directory the asset
// URLs are relative to (so /static/css/style.css resolves to css/style.css).
//
// They are embedded rather than read from disk so that a self-hosted instance
// serves them from the binary alone, whichever directory it is started in.
//
// Only the files are reachable: opening one of the directories that hold them
// reports fs.ErrNotExist (see assetFiles), so a file server built on the result
// answers 404 for a directory path. Walking the tree therefore yields nothing;
// assetsFS is the filesystem to reach for when a caller inside this package
// needs the directories.
//
// [Ja] Assets はビルド済みの CSS と JavaScript を、アセット URL の基準となる
// ディレクトリを根とするファイルシステムとして返します (/static/css/style.css は
// css/style.css に解決されます)。
//
// ディスクから読むのではなく埋め込むのは、セルフホストされたインスタンスが、どの
// ディレクトリで起動してもバイナリだけで配信できるようにするためです。
//
// 到達できるのはファイルだけです。ファイルを収めたディレクトリを開くと
// fs.ErrNotExist になるため (assetFiles を参照)、返り値の上に作ったファイルサーバーは
// ディレクトリのパスに 404 を返します。したがってツリーの走査では何も得られません。
// 本パッケージ内でディレクトリが必要な場合は assetsFS を使います。
func Assets() fs.FS {
	return assetFiles{assetsFS}
}

// assetFiles hides the directories of the filesystem it wraps.
//
// A file server handed the directories generates a listing of everything
// embedded, and that listing is served from a URL that carries no asset version.
// The long lifetime the assets are served with would then pin one listing in the
// browser for a year, where an asset a later deploy adds or removes never shows
// up.
//
// [Ja] assetFiles は、包んだファイルシステムのディレクトリを隠します。
//
// ディレクトリを渡されたファイルサーバーは埋め込んだ内容の一覧を生成しますが、その
// 一覧はアセットバージョンを持たない URL から配信されます。アセットに付ける長い保持
// 期間はその一覧をブラウザに 1 年間留め、後のデプロイが追加・削除したアセットは
// そこに現れません。
type assetFiles struct {
	fs.FS
}

// Open returns the named file, reporting a directory as missing.
//
// [Ja] Open は名前で指定されたファイルを返し、ディレクトリは存在しないものとして
// 扱います。
func (a assetFiles) Open(name string) (fs.File, error) {
	file, err := a.FS.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	if info.IsDir() {
		_ = file.Close()
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	return file, nil
}
