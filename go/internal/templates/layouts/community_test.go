package layouts_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/a-h/templ"

	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// TestCommunity_WithoutRightColumnは、右カラムのないレイアウトがMainの値に
// かかわらず中央カラムをmainランドマークとして描画し、補足のコンテンツ領域を描画
// しないことを検証します。この契約をレイアウト境界で検証するのは、ホームページの正しい
// データが常に中央カラムをMainとし、そのページでは防御的な分岐へ到達できないためです。
func TestCommunity_WithoutRightColumn(t *testing.T) {
	t.Parallel()

	const centerContent = "center-column-content"
	center := templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		_, err := io.WriteString(w, centerContent)
		return err
	})
	data := layouts.CommunityLayoutData{
		Meta: viewmodel.PageMeta{Title: "Community layout test"},
		Columns: layouts.CommunityColumns{
			Center:             center,
			MainLabelledBy:     "community-heading",
			ComplementaryLabel: "complementary-column",
			Main:               layouts.CommunityRightColumn,
		},
	}
	ctx := i18n.SetLocale(context.Background(), model.LocaleEn)

	var buf strings.Builder
	if err := layouts.Community(data).Render(ctx, &buf); err != nil {
		t.Fatalf("Render()のエラー = %v", err)
	}

	got := buf.String()
	main := testutil.Element(t, got, `id="main"`, "</main>")
	if !strings.Contains(main, centerContent) {
		t.Errorf("mainに中央カラムの内容が含まれていない\nmain: %s", main)
	}
	if strings.Contains(got, `aria-label="complementary-column"`) {
		t.Errorf("右カラムが無いのにレイアウトが補足コンテンツの領域を描画している\n出力: %s", got)
	}
}
