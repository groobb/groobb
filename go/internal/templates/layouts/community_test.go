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

// TestCommunity_WithoutRightColumn verifies that a layout without a right
// column renders the center column as its main landmark regardless of Main, and
// does not render a complementary content region. Testing this contract at the
// layout boundary matters because the home page's valid data always names the
// center column as Main and therefore cannot exercise the defensive branch
// itself.
//
// [Ja] TestCommunity_WithoutRightColumn は、右カラムのないレイアウトが Main の値に
// かかわらず中央カラムを main ランドマークとして描画し、補足のコンテンツ領域を描画
// しないことを検証します。この契約をレイアウト境界で検証するのは、ホームページの正しい
// データが常に中央カラムを Main とし、そのページでは防御的な分岐へ到達できないためです。
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
		t.Fatalf("Render() error = %v", err)
	}

	got := buf.String()
	main := testutil.Element(t, got, `id="main"`, "</main>")
	if !strings.Contains(main, centerContent) {
		t.Errorf("main does not contain the center column content\nmain: %s", main)
	}
	if strings.Contains(got, `aria-label="complementary-column"`) {
		t.Errorf("layout renders a complementary content region without a right column\noutput: %s", got)
	}
}
