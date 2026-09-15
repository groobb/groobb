// staticパッケージは、サーバーバイナリに同梱するビルド済みフロントエンド
// アセットを埋め込みます。
package static

import (
	"embed"
	"io/fs"
)

// `all:` 接頭辞は、`pnpm build` がstyle.cssとmain.jsを書き出す前でもパターンが
// 一致するようにするためのものです。ビルド前の2つのディレクトリには .gitkeep
// プレースホルダーしか無く、go:embedは埋め込めるファイルを1つも得られない
// ディレクトリを拒否するため、クローン直後の `go build` や `go test` が壊れます。
//
//go:embed all:css all:js
var assetsFS embed.FS

// Assetsはビルド済みのCSSとJavaScriptを、アセットURLの基準となる
// ディレクトリを根とするファイルシステムとして返します (/static/css/style.cssは
// css/style.cssに解決されます)。
//
// ディスクから読むのではなく埋め込むのは、セルフホストされたインスタンスが、どの
// ディレクトリで起動してもバイナリだけで配信できるようにするためです。
//
// 到達できるのはファイルだけです。ファイルを収めたディレクトリを開くと
// fs.ErrNotExistになるため (assetFilesを参照)、返り値の上に作ったファイルサーバーは
// ディレクトリのパスに404を返します。したがってツリーの走査では何も得られません。
// 本パッケージ内でディレクトリが必要な場合はassetsFSを使います。
func Assets() fs.FS {
	return assetFiles{assetsFS}
}

// assetFilesは、包んだファイルシステムのディレクトリを隠します。
//
// ディレクトリを渡されたファイルサーバーは埋め込んだ内容の一覧を生成しますが、その
// 一覧はアセットバージョンを持たないURLから配信されます。アセットに付ける長い保持
// 期間はその一覧をブラウザに1年間留め、後のデプロイが追加・削除したアセットは
// そこに現れません。
type assetFiles struct {
	fs.FS
}

// Openは名前で指定されたファイルを返し、ディレクトリは存在しないものとして
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
