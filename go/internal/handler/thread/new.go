package thread

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/groobb/groobb/go/internal/httpredirect"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/templates/components"
	"github.com/groobb/groobb/go/internal/templates/layouts"
	threadpage "github.com/groobb/groobb/go/internal/templates/pages/thread"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

// New GET /b/{slug}/threads/new - スレッドを立てるフォームを、コミュニティの
// シェルの中に描画します。書き込めるのはサインイン済みの訪問者だけであるため、
// RequireAuthの背後に登録します。匿名のリクエストはこのアドレスを載せてサインインへ
// 追い返され、トップページではなくこのフォームへ戻ってきます。
//
// フォームを描く前に掲示板を解決します。どの掲示板も指さないslugには、どこへも送信
// できないフォームではなく404ページで応答し、DBのNOCASE照合で解決できる大文字小文字
// 違いのslugは保存済みの小文字slugへリダイレクトします。掲示板自身のページも同じ
// 正規化を行うため、訪問者がどう辿り着いてもフォームは1つのアドレスで開かれます。
//
// このページはnoindexを持ち、no-storeで送ります。認証の背後にあるフォームであり、
// 検索結果が見せるものはそこに無く、再描画されたものは訪問者が打った内容を持つためです。
func (h *Handler) New(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	slug := chi.URLParam(r, "slug")

	resolved, err := h.getBoardUC.Execute(ctx, usecase.GetBoardInput{Slug: slug})
	if err != nil {
		var ae *model.AppError
		if errors.As(err, &ae) && ae.Code == model.AppErrCodeResourceNotFound {
			h.errorRenderer.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "掲示板の取得に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	board := resolved.Board
	if slug != board.Slug {
		httpredirect.ToCanonical(w, r, templates.BoardThreadsNewPath(board.Slug))
		return
	}

	if err := h.renderNew(w, r, http.StatusOK, resolved, newForm{Language: string(uiThreadLanguage(ctx))}); err != nil {
		slog.ErrorContext(ctx, "スレッド作成ページの描画に失敗", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// newFormは、レスポンスが運ぶ作成フォームです。現在のフィールドの値と、それらの
// 何が問題かを持ちます。最初の描画も、拒否された送信の再描画も、いずれもこれを渡すため、
// 2つは描くものではなく運ぶ値で異なります。
//
// Languageは送信された値を持ち、既知の選択を再描画時にも保ちます。不正な値は
// UseCaseが拒否し、フォームのどの選択肢にも一致しません。
//
// Bodyはアプリケーションが扱う形の改行を持ちます。正規化するのは送信を読む側です。
// フォームはUseCaseに渡すのと同じ値から描かれるため、再描画が見せるものは、保存が
// 行われたなら保存されていたものと一致します。
type newForm struct {
	Title    string
	Language string
	Body     string
	Errors   *model.ValidationError
}

// renderNewは、解決済みの掲示板の作成フォームをstatusで書き出します。formが運ぶ
// フィールドとメッセージを載せます。Newはこれで空のフォームを描き、Createは送信が拒否
// されたときに送信されたフォームを描き直します。
//
// サイドバーのサインインのリンクは、応答中のリクエストのアドレスではなくフォーム自身の
// アドレスを指します。Createが応答するのはフォームの送信先のアドレスであり、そこは送信
// しか受け付けません。サインインへ送られてそこへ戻された訪問者は、POSTしか受け付けない
// URLにGETで辿り着くことになります。
//
// エラーを返すのは、レスポンスが書き出される前に起きた失敗、すなわち呼び出し元がまだ500で
// 応答できる失敗に限ります。描画中の失敗はここでログに記録します。ステータスとヘッダーは
// 既に送出の途上にあり、それを別の何かに変える余地が残っていないためです。
func (h *Handler) renderNew(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	resolved *usecase.GetBoardOutput,
	form newForm,
) error {
	ctx := r.Context()

	nav, err := h.getCommunityNavigationUC.Execute(ctx, usecase.GetCommunityNavigationInput{
		UserID: middleware.UserIDFromContext(ctx),
	})
	if err != nil {
		return fmt.Errorf("コミュニティのナビゲーションの取得に失敗: %w", err)
	}

	board := resolved.Board

	meta := viewmodel.DefaultPageMeta(ctx, h.cfg)
	meta.Title = i18n.T(ctx, "thread_new_title")
	meta.NoIndex = true

	returnTo := middleware.SanitizeReturnTo(templates.BoardThreadsNewPath(board.Slug).String())
	sidebar := viewmodel.NewSidebar(nav, middleware.UserFromContext(ctx), middleware.CSRFTokenFromContext(ctx), returnTo)

	pageData := threadpage.NewPageData{
		CSRFToken:           middleware.CSRFTokenFromContext(ctx),
		BoardSlug:           board.Slug,
		BoardName:           board.Name,
		Breadcrumb:          newBreadcrumb(ctx, resolved),
		Title:               form.Title,
		Languages:           newLanguages(model.ThreadLanguage(form.Language)),
		Body:                form.Body,
		PostIntervalSeconds: int(model.PostInterval.Seconds()),
		FormErrors:          form.Errors,
	}
	columns := layouts.CommunityColumns{
		Center:         threadpage.New(pageData),
		MainLabelledBy: threadpage.NewHeadingID,
		Main:           layouts.CommunityCenterColumn,
	}
	layoutData := layouts.CommunityLayoutData{Meta: meta, Sidebar: sidebar, Columns: columns}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	if err := layouts.Community(layoutData).Render(ctx, w); err != nil {
		slog.ErrorContext(ctx, "スレッド作成ページのレンダリングに失敗", "error", err)
	}

	return nil
}

// newBreadcrumbは、これからスレッドを立てる場所を示す経路を組み立てます。掲示板を
// 並べるカテゴリー、掲示板自身、そして訪問者が今いる段としてのフォームです。掲示板の段が
// フォームから出る道でもあります。そこは訪問者が来たページであり、新しいスレッドが現れる
// ページだからです。
//
// 掲示板がどのカテゴリーにも属さないときは (ADR 0011) 最初の段が落ち、経路は掲示板から
// 始まります。掲示板のページと違って空になることはありません。フォームの上位である掲示板は
// 常に名指せるためです。
//
// ベースURLは渡さないため、この経路は構造化データを公開しません。そのデータは検索結果が
// ページの在り処を示せるようにするためのものであり、このページはインデックスされないよう
// 求めています。
func newBreadcrumb(ctx context.Context, resolved *usecase.GetBoardOutput) components.BreadcrumbData {
	items := make([]components.BreadcrumbItem, 0, 3)
	if resolved.Category != nil {
		items = append(items, components.BreadcrumbItem{
			Name: resolved.Category.Name,
			Path: templates.CategoryPath(resolved.Category.Slug),
		})
	}
	items = append(items,
		components.BreadcrumbItem{Name: resolved.Board.Name, Path: templates.BoardPath(resolved.Board.Slug)},
		components.BreadcrumbItem{Name: i18n.T(ctx, "thread_new_heading")},
	)

	return components.BreadcrumbData{Items: items}
}

// uiThreadLanguageはフォームが開いたときに選ばれているスレッド言語、すなわちページが
// 描かれている言語を返します。ある言語でコミュニティを読んでいる訪問者は、その言語で書く
// 見込みが最も高いため、どのスレッドも未回答の選択から始まるようにするのではなく、それを
// 出発点として差し出します。
//
// あくまで出発点にすぎません。送信されたフォームは選ばれた値を持って戻ってきます。再描画で
// UIの言語を選び直すと、訪問者が行った選択を黙って取り消すことになるためです。
func uiThreadLanguage(ctx context.Context) model.ThreadLanguage {
	return i18n.GetLocale(ctx).ThreadLanguage()
}

// newLanguagesは、スレッドを書ける言語をselectが差し出す選択肢へ変換し、フォームが
// 開いたときに選ばれているものに印を付けます。
//
// 集合をここに書き並べずモデルから得るのは、アプリケーションに足された言語がこのフォームを
// 編集せずに現れるようにするためであり、フォームが差し出すものとvalidatorが受け付けるものを
// 一致させるためです。
func newLanguages(selected model.ThreadLanguage) []threadpage.NewLanguage {
	languages := model.ThreadLanguages()

	choices := make([]threadpage.NewLanguage, len(languages))
	for i, language := range languages {
		choices[i] = threadpage.NewLanguage{
			Value:    string(language),
			Language: viewmodel.NewThreadLanguage(language),
			Selected: language == selected,
		}
	}
	return choices
}
