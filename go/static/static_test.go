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

// TestAssets_EmbedsEveryAssetOnDiskは、アセットのビルドがこのディレクトリ配下へ
// 書き出すすべてのファイルが、同じ内容でAssets経由に届くことを検証します。
//
// 固定のファイル一覧ではなくディレクトリを走査するのは、embedのパターンから外れた
// アセット (例えば画像用の新しいサブディレクトリ) を、ビルドしたバイナリで404になる
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

		// このパッケージのGoソースはアセットの隣にあるが、配信対象ではなく
		// コンパイル対象である。
		if filepath.Ext(path) == ".go" {
			return nil
		}

		name := filepath.ToSlash(path)

		embedded, err := fs.ReadFile(assets, name)
		if err != nil {
			t.Errorf("%s が埋め込まれていない: %v", name, err)
			return nil
		}

		onDisk, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if !bytes.Equal(embedded, onDisk) {
			t.Errorf("埋め込まれた %s (%d バイト) がディスク上のファイル (%d バイト) と異なる", name, len(embedded), len(onDisk))
		}

		return nil
	})
	if err != nil {
		t.Fatalf("アセットのディレクトリの走査に失敗: %v", err)
	}
}

// TestAssets_RootsPathsAtTheAssetURLは、返されるファイルシステムが /staticの
// URL接頭辞が期待する位置を根としており、2つのアセットディレクトリへ先頭のパス要素
// なしで到達できることを検証します。
//
// style.cssとmain.jsではなく .gitkeepプレースホルダーを見るのは、前者2つが
// アセットのビルド後にしか存在せず、本検証はクローン直後にも成り立つ必要があるためです。
func TestAssets_RootsPathsAtTheAssetURL(t *testing.T) {
	t.Parallel()

	assets := Assets()

	for _, name := range []string{"css/.gitkeep", "js/.gitkeep"} {
		if _, err := fs.Stat(assets, name); err != nil {
			t.Errorf("fs.Stat(%q)に失敗: %v", name, err)
		}
	}
}

// TestAssets_HidesDirectoriesは、Assetsの上に作ったファイルサーバーが、
// バイナリの埋め込み内容を一覧にする代わりにディレクトリへ404を返すことを検証します。
//
// 一覧はアセットバージョンを持たないURLから配信されるため、アセットに付ける長い
// 保持期間が古い一覧をブラウザに1年間残してしまいます。
func TestAssets_HidesDirectories(t *testing.T) {
	t.Parallel()

	server := http.StripPrefix("/static", http.FileServer(http.FS(Assets())))

	tests := []struct {
		name   string
		target string
		want   int
	}{
		{
			name:   "アセットのルート",
			target: "/static/",
			want:   http.StatusNotFound,
		},
		{
			name:   "末尾にスラッシュが付いたディレクトリ",
			target: "/static/css/",
			want:   http.StatusNotFound,
		},
		{
			name:   "末尾にスラッシュの無いディレクトリ",
			target: "/static/css",
			want:   http.StatusNotFound,
		},
		{
			name:   "ディレクトリ内のファイル",
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
				t.Errorf("GET %s = %d、期待値 = %d", tt.target, rec.Code, tt.want)
			}
		})
	}
}
