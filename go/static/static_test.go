package static

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestAssets_EmbedsEveryAssetOnDisk verifies that every file the asset build
// writes under this directory is reachable through Assets with the same bytes.
//
// It walks the directory instead of asserting on a fixed list so that an asset
// added outside the embed patterns (a new subdirectory for images, say) fails
// here rather than 404ing only in a built binary. Comparing against the files on
// disk is deterministic because changing an embedded file invalidates the build
// cache, so the test binary always carries what the walk finds.
//
// [Ja] TestAssets_EmbedsEveryAssetOnDisk は、アセットのビルドがこのディレクトリ配下へ
// 書き出すすべてのファイルが、同じ内容で Assets 経由に届くことを検証します。
//
// 固定のファイル一覧ではなくディレクトリを走査するのは、embed のパターンから外れた
// アセット (例えば画像用の新しいサブディレクトリ) を、ビルドしたバイナリで 404 になる
// 前にここで失敗させるためです。埋め込みファイルを変更するとビルドキャッシュが無効化され、
// テストバイナリは常に走査結果と同じものを持つため、ディスク上のファイルとの比較は
// 決定的になります。
func TestAssets_EmbedsEveryAssetOnDisk(t *testing.T) {
	t.Parallel()

	assets := Assets()

	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		// The Go sources of this package sit next to the assets and are compiled,
		// not served.
		//
		// [Ja] このパッケージの Go ソースはアセットの隣にあるが、配信対象ではなく
		// コンパイル対象である。
		if filepath.Ext(path) == ".go" {
			return nil
		}

		name := filepath.ToSlash(path)

		embedded, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Errorf("%s is not embedded: %v", name, err)
			return nil
		}

		onDisk, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if !bytes.Equal(embedded, onDisk) {
			t.Errorf("embedded %s (%d bytes) differs from the file on disk (%d bytes)", name, len(embedded), len(onDisk))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk the asset directory: %v", err)
	}
}

// TestAssets_RootsPathsAtTheAssetURL verifies that the returned filesystem is
// rooted where the /static URL prefix expects it, so that the two asset
// directories are reachable without a leading path segment.
//
// It stats the .gitkeep placeholders rather than style.css and main.js because
// those two exist only after an asset build, and this has to hold on a fresh
// clone as well.
//
// [Ja] TestAssets_RootsPathsAtTheAssetURL は、返されるファイルシステムが /static の
// URL 接頭辞が期待する位置を根としており、2 つのアセットディレクトリへ先頭のパス要素
// なしで到達できることを検証します。
//
// style.css と main.js ではなく .gitkeep プレースホルダーを見るのは、前者 2 つが
// アセットのビルド後にしか存在せず、本検証はクローン直後にも成り立つ必要があるためです。
func TestAssets_RootsPathsAtTheAssetURL(t *testing.T) {
	t.Parallel()

	assets := Assets()

	for _, name := range []string{"css/.gitkeep", "js/.gitkeep"} {
		if _, err := fs.Stat(assets, name); err != nil {
			t.Errorf("fs.Stat(%q) failed: %v", name, err)
		}
	}
}

// TestAssets_HidesDirectories verifies that a file server built on Assets
// answers 404 for a directory instead of listing what the binary embeds.
//
// A listing is served from a URL that carries no asset version, so the long
// lifetime the assets are served with would keep a stale one in the browser for
// a year.
//
// [Ja] TestAssets_HidesDirectories は、Assets の上に作ったファイルサーバーが、
// バイナリの埋め込み内容を一覧にする代わりにディレクトリへ 404 を返すことを検証します。
//
// 一覧はアセットバージョンを持たない URL から配信されるため、アセットに付ける長い
// 保持期間が古い一覧をブラウザに 1 年間残してしまいます。
func TestAssets_HidesDirectories(t *testing.T) {
	t.Parallel()

	server := http.StripPrefix("/static", http.FileServer(http.FS(Assets())))

	tests := []struct {
		name   string
		target string
		want   int
	}{
		{
			name:   "the asset root",
			target: "/static/",
			want:   http.StatusNotFound,
		},
		{
			name:   "a directory with a trailing slash",
			target: "/static/css/",
			want:   http.StatusNotFound,
		},
		{
			name:   "a directory without a trailing slash",
			target: "/static/css",
			want:   http.StatusNotFound,
		},
		{
			name:   "a file inside a directory",
			target: "/static/css/.gitkeep",
			want:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.target, nil))

			if rec.Code != tt.want {
				t.Errorf("GET %s = %d, want %d", tt.target, rec.Code, tt.want)
			}
		})
	}
}
