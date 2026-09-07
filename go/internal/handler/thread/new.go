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

// New GET /b/{slug}/threads/new - renders the form a thread is started from,
// inside the community shell. Only a signed-in visitor can write, so it is
// registered behind RequireAuth: an anonymous request is turned away to sign-in
// carrying this address, and comes back to the form rather than to the top page.
//
// The board is resolved before the form is drawn, so a slug naming no board is
// answered with the 404 page instead of a form that would post nowhere, and a
// case variant that resolves through the database's NOCASE collation is
// redirected to the stored lowercase slug. The board's own page normalizes the
// same way, so the form is reached under one address however the visitor arrived
// at it.
//
// The page carries noindex and is sent no-store. It is a form behind
// authentication, so there is nothing on it for a search result to show, and a
// re-rendered one holds what the visitor typed.
//
// [Ja] New GET /b/{slug}/threads/new - スレッドを立てるフォームを、コミュニティの
// シェルの中に描画します。書き込めるのはサインイン済みの訪問者だけであるため、
// RequireAuth の背後に登録します。匿名のリクエストはこのアドレスを載せてサインインへ
// 追い返され、トップページではなくこのフォームへ戻ってきます。
//
// フォームを描く前に掲示板を解決します。どの掲示板も指さない slug には、どこへも送信
// できないフォームではなく 404 ページで応答し、DB の NOCASE 照合で解決できる大文字小文字
// 違いの slug は保存済みの小文字 slug へリダイレクトします。掲示板自身のページも同じ
// 正規化を行うため、訪問者がどう辿り着いてもフォームは 1 つのアドレスで開かれます。
//
// このページは noindex を持ち、no-store で送ります。認証の背後にあるフォームであり、
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

// newForm is the creation form as a response carries it: the fields as they
// stand, and what is wrong with them. The first render and every re-render of a
// refused submission pass one, so the two differ in the values they carry rather
// than in what they draw.
//
// Language keeps the submitted value so known choices survive a re-render.
// Invalid values are rejected by the UseCase; they match none of the options
// offered by the form.
//
// Body holds the line endings the application works in, normalized by whoever
// reads the submission. The form is drawn from the same value the UseCase is
// given, so what a re-render shows is what a save would have stored.
//
// [Ja] newForm は、レスポンスが運ぶ作成フォームです。現在のフィールドの値と、それらの
// 何が問題かを持ちます。最初の描画も、拒否された送信の再描画も、いずれもこれを渡すため、
// 2 つは描くものではなく運ぶ値で異なります。
//
// Language は送信された値を持ち、既知の選択を再描画時にも保ちます。不正な値は
// UseCase が拒否し、フォームのどの選択肢にも一致しません。
//
// Body はアプリケーションが扱う形の改行を持ちます。正規化するのは送信を読む側です。
// フォームは UseCase に渡すのと同じ値から描かれるため、再描画が見せるものは、保存が
// 行われたなら保存されていたものと一致します。
type newForm struct {
	Title    string
	Language string
	Body     string
	Errors   *model.ValidationError
}

// renderNew writes the creation form of the resolved board at status, carrying
// the fields and messages of form. New draws the empty form with it, and Create
// redraws the submitted one when the submission is refused.
//
// The sidebar's sign-in links point back at the form's own address rather than
// at the address of the request being answered. Create answers under the address
// the form posts to, which accepts nothing but a submission: a visitor sent to
// sign-in and returned there would arrive at a POST-only URL with a GET.
//
// An error is returned only for a failure that happens before the response is
// written, which the caller can still answer with a 500. A failure during
// rendering is logged here instead, since the status and headers are already on
// their way and there is nothing left to turn it into.
//
// [Ja] renderNew は、解決済みの掲示板の作成フォームを status で書き出します。form が運ぶ
// フィールドとメッセージを載せます。New はこれで空のフォームを描き、Create は送信が拒否
// されたときに送信されたフォームを描き直します。
//
// サイドバーのサインインのリンクは、応答中のリクエストのアドレスではなくフォーム自身の
// アドレスを指します。Create が応答するのはフォームの送信先のアドレスであり、そこは送信
// しか受け付けません。サインインへ送られてそこへ戻された訪問者は、POST しか受け付けない
// URL に GET で辿り着くことになります。
//
// エラーを返すのは、レスポンスが書き出される前に起きた失敗、すなわち呼び出し元がまだ 500 で
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

	nav, err := h.getCommunityNavigationUC.Execute(ctx)
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

// newBreadcrumb builds the trail naming where the thread is about to be started:
// the category that lists the board, the board itself, and the form as the step
// the visitor is on. The board's step is the way back out of the form, since it
// is the page the visitor came from and the one the new thread will appear on.
//
// A board sitting in no category (ADR 0011) drops the first step, and the trail
// starts at the board. Unlike a board's own page it is never left empty: the
// board above the form is always there to name.
//
// No base URL is passed, so the trail publishes no structured data. That data
// exists to let a search result show where a page sits, and this page asks not
// to be indexed.
//
// [Ja] newBreadcrumb は、これからスレッドを立てる場所を示す経路を組み立てます。掲示板を
// 並べるカテゴリー、掲示板自身、そして訪問者が今いる段としてのフォームです。掲示板の段が
// フォームから出る道でもあります。そこは訪問者が来たページであり、新しいスレッドが現れる
// ページだからです。
//
// 掲示板がどのカテゴリーにも属さないときは (ADR 0011) 最初の段が落ち、経路は掲示板から
// 始まります。掲示板のページと違って空になることはありません。フォームの上位である掲示板は
// 常に名指せるためです。
//
// ベース URL は渡さないため、この経路は構造化データを公開しません。そのデータは検索結果が
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

// uiThreadLanguage returns the thread language the form opens with: the one the
// page is being drawn in. A visitor reading the community in a language is the
// one most likely to write in it, so it is offered as the starting point rather
// than making every thread begin with an unanswered choice.
//
// It is only ever the starting point. A submitted form comes back carrying what
// was chosen, since re-selecting the UI language on a re-render would quietly
// undo a choice the visitor made.
//
// [Ja] uiThreadLanguage はフォームが開いたときに選ばれているスレッド言語、すなわちページが
// 描かれている言語を返します。ある言語でコミュニティを読んでいる訪問者は、その言語で書く
// 見込みが最も高いため、どのスレッドも未回答の選択から始まるようにするのではなく、それを
// 出発点として差し出します。
//
// あくまで出発点にすぎません。送信されたフォームは選ばれた値を持って戻ってきます。再描画で
// UI の言語を選び直すと、訪問者が行った選択を黙って取り消すことになるためです。
func uiThreadLanguage(ctx context.Context) model.ThreadLanguage {
	return i18n.GetLocale(ctx).ThreadLanguage()
}

// newLanguages converts the languages a thread may be written in into the
// choices the select offers, marking selected as the one the form opens with.
//
// The set comes from the model rather than being listed here, so that a language
// added to the application appears in this form without it being edited, and the
// form offers exactly what the validator accepts.
//
// [Ja] newLanguages は、スレッドを書ける言語を select が差し出す選択肢へ変換し、フォームが
// 開いたときに選ばれているものに印を付けます。
//
// 集合をここに書き並べずモデルから得るのは、アプリケーションに足された言語がこのフォームを
// 編集せずに現れるようにするためであり、フォームが差し出すものと validator が受け付けるものを
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
