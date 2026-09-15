package post_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"golang.org/x/net/html"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/handler/post"
	"github.com/groobb/groobb/go/internal/handler/thread"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/testutil"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
)

const (
	communityName = "ジャズ喫茶"
	appURL        = "https://example.com"
)

// fixtureはケースが送信を行うコミュニティです。投稿を1つ持つ開いたスレッド、
// 既に持てる投稿をすべて持っているスレッド、そして両者が立っている掲示板です。
type fixture struct {
	db      *database.DB
	board   model.BoardID
	open    model.ThreadID
	quiet   model.ThreadID
	full    model.ThreadID
	handler *post.Handler
}

// newFixtureは、各ケースが送信を行うコミュニティを組み立てます。スレッドの投稿は
// 誰のものでもないため、ケースがサインインするアカウントはまだ何も書いておらず、1人の
// 投稿と投稿の間隔に阻まれることがありません。
//
// 掲示板は、最後の投稿が開いたスレッドのそれより新しい2つ目のスレッドを持ちます。返信が
// それを書いたスレッドを掲示板の一覧の中でどこへ動かすかを、ケースが述べられるようにする
// ためです。
func newFixture(t *testing.T) fixture {
	t.Helper()

	ctx := context.Background()
	db := testutil.SetupDB(t)

	if _, err := db.Writer.ExecContext(ctx, "INSERT INTO communities (id, name) VALUES (1, ?)", communityName); err != nil {
		t.Fatalf("communitiesへのINSERTに失敗: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)

	music, err := categoryRepo.Create(ctx, repository.CreateCategoryInput{Slug: "music", Name: "音楽"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}
	board, err := boardRepo.Create(ctx, repository.CreateBoardInput{CategoryID: &music.ID, Slug: "jazz", Name: "ジャズ・ファンク"})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	open := newThread(t, db, board.ID, "枯葉の名演", 1, time.Now().Add(-2*time.Hour))
	quiet := newThread(t, db, board.ID, "最近買ったレコード", 1, time.Now().Add(-1*time.Hour))
	full := newThread(t, db, board.ID, "埋まったスレッド", model.ThreadPostLimit, time.Now().Add(-48*time.Hour))

	return fixture{db: db, board: board.ID, open: open, quiet: quiet, full: full, handler: newHandlerForDB(db)}
}

// newThreadは投稿を1つ持つスレッドを掲示板へ追加し、そのスレッドが持つ投稿の件数を
// 述べます。1000行を書かずにスレッドを上限に置けるようにするためです。
func newThread(t *testing.T, db *database.DB, boardID model.BoardID, title string, postsCount int, lastPostedAt time.Time) model.ThreadID {
	t.Helper()

	ctx := context.Background()
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)

	thread, err := threadRepo.Create(ctx, repository.CreateThreadInput{
		BoardID:  boardID,
		Title:    title,
		Language: model.LocaleJa.ThreadLanguage(),
	})
	if err != nil {
		t.Fatalf("テスト用スレッドの作成に失敗: %v", err)
	}
	first, err := postRepo.Create(ctx, repository.CreatePostInput{ThreadID: thread.ID, Number: 1, Body: "最初の投稿"})
	if err != nil {
		t.Fatalf("テスト用投稿の作成に失敗: %v", err)
	}
	if err := threadRepo.UpdateLastPost(ctx, thread.ID, repository.UpdateThreadLastPostInput{
		PostsCount:   postsCount,
		LastPostID:   first.ID,
		LastPostedAt: lastPostedAt,
	}); err != nil {
		t.Fatalf("テスト用スレッドの集計の更新に失敗: %v", err)
	}

	return thread.ID
}

// newHandlerForDBは、渡されたアプリケーションデータベース上にpost Handlerを
// 構築します。
func newHandlerForDB(db *database.DB) *post.Handler {
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(
		repository.NewCommunityRepository(db),
		repository.NewBoardRepository(db),
		repository.NewRoleRepository(db),
	)
	getThreadSummaryUC := usecase.NewGetThreadSummaryUsecase(repository.NewThreadRepository(db))
	createPostUC := usecase.NewCreatePostUsecase(
		db.Writer,
		validator.NewPostCreateValidator(),
		repository.NewThreadRepository(db),
		repository.NewPostRepository(db),
		repository.NewPostReferenceRepository(db),
		repository.NewUserRepository(db),
	)

	cfg := &config.Config{Env: "dev", AppURL: appURL}
	return post.NewHandler(cfg, httperror.NewRenderer(cfg), getCommunityNavigationUC, getThreadSummaryUC, createPostUC)
}

// newAuthorはdbにアカウントを追加し、セッションが名指す形で返します。投稿が
// 帰属するidと、シェルが表示するatnameです。
func newAuthor(t *testing.T, db *database.DB, atname string) *model.User {
	t.Helper()

	id := testutil.NewUserBuilder(t, db).WithAtname(atname).WithEmail(atname + "@example.com").Build()
	return &model.User{ID: id, Atname: atname}
}

// newSubmitRequestは、ルーターがハンドラーへ渡すのと同じ形でPOST /t/{id}/postsの
// リクエストを組み立てます。フォームはそれが送信されるボディとして、idはchiのルート
// contextに、ロケール・現在のパス・書き手はリクエストcontextに、i18n・templates・認証の
// 各ミドルウェアがするのと同じように直接置きます。
func newSubmitRequest(t *testing.T, id string, locale model.Locale, author *model.User, form url.Values) *http.Request {
	t.Helper()

	path := "/t/" + id + "/posts"
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	ctx := i18n.SetLocale(req.Context(), locale)
	ctx = templates.SetCurrentPath(ctx, path)
	ctx = viewmodel.SetSiteName(ctx, communityName)
	ctx = middleware.SetUserToContext(ctx, author)

	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", id)
	ctx = context.WithValue(ctx, chi.RouteCtxKey, routeCtx)

	return req.WithContext(ctx)
}

// replyは検証を通る送信であり、フォームそのものではなく、整った返信の周りで何が
// 起きるかを問うケースのためのものです。
func reply(body string) url.Values {
	return url.Values{"body": {body}}
}

// savedPostはデータベースが持つ投稿を、行が述べるとおりに読み戻したものです。
type savedPost struct {
	id       model.PostID
	number   int
	body     string
	authorID *model.UserID
}

// readLatestPostは、スレッドの中でレス番号が最も大きい投稿、すなわち送信がたった今
// 加えたものを読み戻します。
func readLatestPost(t *testing.T, db *database.DB, threadID model.ThreadID) savedPost {
	t.Helper()

	var saved savedPost
	err := db.Reader.QueryRowContext(context.Background(), `
		SELECT id, number, body, user_id FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1
	`, int64(threadID)).Scan(&saved.id, &saved.number, &saved.body, &saved.authorID)
	if err != nil {
		t.Fatalf("保存された投稿の読み戻しに失敗: %v", err)
	}

	return saved
}

func countPosts(t *testing.T, db *database.DB, threadID model.ThreadID) int {
	t.Helper()

	var count int
	err := db.Reader.QueryRowContext(context.Background(), "SELECT count(*) FROM posts WHERE thread_id = ?", int64(threadID)).Scan(&count)
	if err != nil {
		t.Fatalf("投稿の件数の取得に失敗: %v", err)
	}

	return count
}

// TestCreateは、整った返信が、それが書かれたスレッドへ加えられ、その書き手がそこへ
// 送られることを検証します。HTTP 303で、新しい投稿のアンカーを付けたスレッドへ送り、投稿は
// 次のレス番号で保存され、スレッド自身が持つその姿 (件数・最後の投稿・それが届いた時刻) も
// それに合わせて更新されます。
//
// 投稿はセッションが名指すアカウントに帰属します。フォームは書き手を名指すフィールドを
// 持たず、本ケースはそれが読まれないことを述べるために1つ送ります。誰の投稿かを述べうる
// 送信は、他人の名前で署名できてしまうためです。
func TestCreate(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	other := newAuthor(t, f.db, "bob")

	form := reply(">>1 Bill Evansの演奏が好きです")
	form.Set("user_id", other.ID.String())

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, form))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	saved := readLatestPost(t, f.db, f.open)
	wantLocation := templates.ThreadPostAnchorPath(viewmodel.ThreadID(f.open), 2).String()
	if got := rec.Header().Get("Location"); got != wantLocation {
		t.Errorf("Location = %q、期待値 = %q", got, wantLocation)
	}
	if saved.number != 2 {
		t.Errorf("保存されたレス番号 = %d、期待値 = 2", saved.number)
	}
	if saved.body != ">>1 Bill Evansの演奏が好きです" {
		t.Errorf("保存された本文 = %q、期待値 = %q", saved.body, ">>1 Bill Evansの演奏が好きです")
	}
	if saved.authorID == nil || *saved.authorID != author.ID {
		t.Errorf("投稿の作者 = %v、期待値 = %v (セッションのアカウント)", saved.authorID, author.ID)
	}

	thread, err := repository.NewThreadRepository(f.db).FindByID(context.Background(), f.open)
	if err != nil {
		t.Fatalf("スレッドの読み戻しに失敗: %v", err)
	}
	if thread.PostsCount != 2 {
		t.Errorf("スレッドの投稿数 = %d、期待値 = 2", thread.PostsCount)
	}
	if thread.LastPostID == nil || *thread.LastPostID != saved.id {
		t.Errorf("スレッドの最終投稿 = %v、期待値 = %v", thread.LastPostID, saved.id)
	}
}

// TestCreate_MovesTheThreadToTheTopOfItsBoardは、保存された返信に掲示板の一覧が
// 応じることを検証します。返信が書かれたスレッドは、最後の投稿がより古いスレッドの上に
// 立ちます。返信の前には、その下に立っていました。
func TestCreate_MovesTheThreadToTheTopOfItsBoard(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	threadRepo := repository.NewThreadRepository(f.db)

	before, err := threadRepo.ListByBoardID(ctx, f.board)
	if err != nil {
		t.Fatalf("ListByBoardID()のエラー = %v", err)
	}
	if before[0].ID != f.quiet {
		t.Fatalf("返信前の一覧の先頭 = %v、期待値 = %v", before[0].ID, f.quiet)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("久しぶりに聴き返しました")))
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}

	after, err := threadRepo.ListByBoardID(ctx, f.board)
	if err != nil {
		t.Fatalf("ListByBoardID()のエラー = %v", err)
	}
	if after[0].ID != f.open {
		t.Errorf("返信後の一覧の先頭 = %v、期待値 = %v (返信のあったスレッド)", after[0].ID, f.open)
	}
}

// TestCreate_IgnoresQueryStringFieldsは、本文が送信されたボディからのみ読まれる
// ことを検証します。クエリ文字列にそれを載せたリンクは何も書きません。そうでなければ、
// サインイン済みの訪問者がそのリンクを踏んだだけで、何も書いていないのにその人の名前で
// 投稿されることになります。
func TestCreate_IgnoresQueryStringFields(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	req := newSubmitRequest(t, f.open.String(), model.LocaleJa, author, url.Values{})
	req.URL.RawQuery = reply("クエリ文字列からの投稿").Encode()

	rec := httptest.NewRecorder()
	f.handler.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件 (用意した投稿のみ)", got)
	}
	if body := rec.Body.String(); !strings.Contains(body, "入力してください") {
		t.Error("クエリ文字列だけの送信が、空のフォームとして扱われていない")
	}
}

// TestCreate_ValidationErrorは、直すところのある返信が、それを直せるページとして
// 返ってくることを検証します。HTTP 422と、それが属するフィールドに紐づくメッセージ、
// そして書かれたものはフォームに残り、何も2度打たずに済みます。
//
// このページは返信が書かれたスレッドを名指してリンクします。応答するアドレスは訪問者が
// 読めることを何も述べないためです。マークアップに見えるテキストは、ページの一部になるので
// はなく、そのままのテキストとして表示されます。
func TestCreate_ValidationError(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply(strings.Repeat("あ", 10001))))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}
	if got, want := rec.Header().Get("Cache-Control"), "private, no-store"; got != want {
		t.Errorf("Cache-Control = %q、期待値 = %q", got, want)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件 (用意した投稿のみ)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "本文は10,000文字以内で入力してください") {
		t.Error("再描画されたページに本文のエラーメッセージが含まれていない")
	}
	if !strings.Contains(body, `href="`+templates.ThreadPath(viewmodel.ThreadID(f.open)).String()+`"`) {
		t.Error("再描画されたページに投稿先のスレッドへのリンクが含まれていない")
	}
	if !strings.Contains(body, "枯葉の名演") {
		t.Error("再描画されたページに投稿先のスレッドのタイトルが含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), strings.Repeat("あ", 10001)) {
		t.Error("再描画されたフォームに、送信された本文が残っていない")
	}
	if !strings.Contains(body, `action="`+templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()+`"`) {
		t.Error("再描画されたフォームが、元の投稿先へ送信できる形になっていない")
	}
}

// TestCreate_EscapesTheSubmittedBodyは、マークアップに見える本文が、ページの一部
// ではなく、そのままのテキストとして返ってくることを検証します。
func TestCreate_EscapesTheSubmittedBody(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("<script>alert(1)</script>\n"+strings.Repeat("あ", 10001))))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusUnprocessableEntity)
	}

	body := rec.Body.String()
	if strings.Contains(body, "<script>alert(1)</script>") {
		t.Error("送信された本文がマークアップとして描画されている")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("再描画されたフォームに、送信された本文がテキストとして残っていない")
	}
}

// TestCreate_TooSoonAfterTheLastPostは、1人の投稿と投稿の間隔が尽きる前に届いた
// 返信がHTTP 429で拒否されることを検証します。待ち時間はRetry-Afterとしてブラウザにも
// ページ上にも述べられ、返信は、待ち時間が明けたら送り直せるように保たれます。
//
// 拒否の理由は送信そのものであってそれが運ぶフィールドではないため、フォームの上の要約が
// キャレットを取り、本文に落ち度の印は付きません。
func TestCreate_TooSoonAfterTheLastPost(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	first := httptest.NewRecorder()
	f.handler.Create(first, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("最初の返信")))
	if first.Code != http.StatusSeeOther {
		t.Fatalf("1件目のステータスコード = %d、期待値 = %d", first.Code, http.StatusSeeOther)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.quiet.String(), model.LocaleJa, author, reply("続けて書いた返信")))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusTooManyRequests)
	}
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After = %q、期待値は整数の秒数", rec.Header().Get("Retry-After"))
	}
	if wait := int(model.PostInterval.Seconds()); retryAfter <= 0 || retryAfter > wait {
		t.Errorf("Retry-After = %d、期待値は1以上 %d 以下", retryAfter, wait)
	}
	if got := countPosts(t, f.db, f.quiet); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件 (拒否された送信は何も残さない)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "秒後にもう一度お試しください。") {
		t.Error("再描画されたページに、待ち時間を述べる文言が含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "続けて書いた返信") {
		t.Error("拒否された送信の本文がフォームに残っていない")
	}

	summary := testutil.Element(t, body, `tabindex="-1" autofocus`, "</div>")
	if !strings.Contains(summary, "秒後にもう一度お試しください。") {
		t.Errorf("フォーカスを取る要素 = %s、期待値はエラーの要約", summary)
	}
	if control := testutil.OpeningTag(t, body, `id="body"`); strings.Contains(control, "aria-invalid") || strings.Contains(control, "autofocus") {
		t.Errorf("本文の入力欄が不正扱い、またはフォーカスを取っている: %s", control)
	}
}

// TestCreate_LockedThreadは、持てる投稿をすべて持っているスレッドへの返信が
// HTTP 409で拒否されること、送信についてのメッセージの代わりに理由が述べられること、
// そしてテキストが、もう一度拒否されるフォームの中ではなく、読んで写し取れる場所に
// 保たれることを検証します。
//
// スレッドはその送信から何も受け取りません。本ケースは行を確かめ、上限を越える番号の投稿が、
// フォームを介さずアドレスへ直接送ることでも保存できないことを述べます。
func TestCreate_LockedThread(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.full.String(), model.LocaleJa, author, reply("1001件目の投稿")))

	if rec.Code != http.StatusConflict {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusConflict)
	}
	if got := countPosts(t, f.db, f.full); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件 (上限に達したスレッドは投稿を受け付けない)", got)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("ロック中のスレッドへの返信の応答に、上限に達した旨の文言が含まれていない")
	}
	if strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("ロックの案内の傍らに、保存の失敗を述べる汎用の文言が並んでいる")
	}
	if !strings.Contains(body, "1001件目の投稿") {
		t.Error("拒否された送信の本文がページに残っていない")
	}
	if !strings.Contains(body, "この本文はコピーできます。") {
		t.Error("本文を控えられることを伝えるヒントが含まれていない")
	}
	if strings.Contains(body, `action="`+templates.ThreadPostsPath(viewmodel.ThreadID(f.full)).String()+`"`) {
		t.Error("ロック中のスレッドへの再送フォームが描かれている")
	}
	if strings.Contains(body, "返信する") {
		t.Error("ロック中のスレッドの応答に、返信を送信するボタンが描かれている")
	}
}

// TestCreate_WithdrawnAccountは、アカウントが去ったセッションがHTTP 403で拒否
// され、その旨を伝えられることを検証します。もう存在しないアカウントに投稿が帰属すること
// はありません。
func TestCreate_WithdrawnAccount(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	withdrawn := &model.User{
		ID:     testutil.NewUserBuilder(t, f.db).WithAtname("bob").WithDeletedAt(time.Now().Add(-24 * time.Hour)).Build(),
		Atname: "bob",
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, withdrawn, reply("退会後の投稿")))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusForbidden)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件", got)
	}
	if !strings.Contains(rec.Body.String(), "このアカウントでは投稿できません。") {
		t.Error("再描画されたページに、投稿できないことを伝える文言が含まれていない")
	}
}

// TestCreate_UnknownThreadは、どのスレッドも名指さないアドレスへの送信が、そこに
// 無いスレッドのページではなく共通の404ページで応答されることを検証します。そもそも
// スレッドのidを表しえないパスも、ルックアップを伴わずに同じ応答になります。
func TestCreate_UnknownThread(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"999999", "abc", "0", "-1"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			author := newAuthor(t, f.db, "alice")

			rec := httptest.NewRecorder()
			f.handler.Create(rec, newSubmitRequest(t, id, model.LocaleJa, author, reply("どこにも属さない投稿")))

			if rec.Code != http.StatusNotFound {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}
			if !strings.Contains(rec.Body.String(), "ページが見つかりません") {
				t.Error("404ページの文言が含まれていない")
			}
		})
	}
}

// TestCreate_NonCanonicalIDは、アプリケーションが書かない綴りのidでスレッドへ
// 到達した返信が、リダイレクトされずに保存されることを検証します。リダイレクトはPOSTを
// GETに変え、書かれたものを落としてしまいます。正規のアドレスは、保存に続く303で
// 届きます。
func TestCreate_NonCanonicalID(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, "0"+f.open.String(), model.LocaleJa, author, reply("先頭にゼロの付いたアドレスからの返信")))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	want := templates.ThreadPostAnchorPath(viewmodel.ThreadID(f.open), 2).String()
	if got := rec.Header().Get("Location"); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got := countPosts(t, f.db, f.open); got != 2 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 2件", got)
	}
}

// TestCreate_SaveFailureは、アプリケーションが保存できなかった返信が、それが書かれた
// フォームとして (HTTP 500) 返ってくることを検証します。訪問者が書いたものを保つため、
// 打ち直さずにもう一度試せます。
func TestCreate_SaveFailure(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	author := newAuthor(t, f.db, "alice")
	if err := f.db.Writer.Close(); err != nil {
		t.Fatalf("WriterのClose()のエラー = %v", err)
	}

	rec := httptest.NewRecorder()
	f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, reply("保存できない返信")))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusInternalServerError)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "投稿を保存できませんでした。") {
		t.Error("再描画されたページに、保存できなかったことを伝える文言が含まれていない")
	}
	if !strings.Contains(testutil.Element(t, body, `id="body"`, "</textarea>"), "保存できない返信") {
		t.Error("保存できなかった送信の本文がフォームに残っていない")
	}
}

// TestCreate_ThroughRouterは、配信される形でのルートを検証します。ボディの上限・
// CSRF検証・RequireAuthの背後にあり、書き手はフォームが運ぶ何かではなくセッション
// Cookieから解決されます。
//
// トークンを伴わない送信は、ハンドラーが走る前に拒否され、何も保存しません。他のサイトから
// 送信されたフォームがその形になります。
func TestCreate_ThroughRouter(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	tests := []struct {
		name       string
		withCSRF   bool
		wantStatus int
		wantPosts  int
	}{
		{name: "正常系: CSRFトークンを伴う送信は保存される", withCSRF: true, wantStatus: http.StatusSeeOther, wantPosts: 2},
		{name: "異常系: CSRFトークンの無い送信は拒否される", withCSRF: false, wantStatus: http.StatusForbidden, wantPosts: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			author := newAuthor(t, f.db, "alice")
			sessionToken := "session-token-" + strconv.Itoa(tt.wantPosts)
			testutil.NewUserSessionBuilder(t, f.db).WithUserID(author.ID).WithToken(sessionToken).Build()

			cfg := &config.Config{Env: "test"}
			auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), cfg))
			router := chi.NewRouter()
			router.Use(middleware.PostFormLimit)
			router.Use(middleware.NewCSRF(cfg).Middleware)
			router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

			form := reply("ルーター越しの返信")
			if tt.withCSRF {
				form.Set("csrf_token", csrfToken)
			}

			path := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
			req.AddCookie(&http.Cookie{Name: session.CookieName, Value: sessionToken})
			req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, tt.wantStatus)
			}
			if got := countPosts(t, f.db, f.open); got != tt.wantPosts {
				t.Errorf("スレッドの投稿 = %d 件、期待値 = %d 件", got, tt.wantPosts)
			}
			if tt.wantPosts == 1 {
				return
			}

			saved := readLatestPost(t, f.db, f.open)
			want := templates.ThreadPostAnchorPath(viewmodel.ThreadID(f.open), saved.number).String()
			if got := rec.Header().Get("Location"); got != want {
				t.Errorf("Location = %q、期待値 = %q", got, want)
			}
			if saved.authorID == nil || *saved.authorID != author.ID {
				t.Errorf("投稿の作者 = %v、期待値 = %v (セッションのアカウント)", saved.authorID, author.ID)
			}
		})
	}
}

// TestCreate_ExpiredSessionは、それが書かれたときのセッションが失われた後に送られた
// 返信が、アカウントへ戻る道で応答されること、そして書かれた先のスレッドがそのまま残ることを
// 検証します。
//
// 送信が届くアドレスは送信しか受け付けないため、RequireAuthはreturn_toを伴わない
// サインインへ送ります。ここへ連れ戻された訪問者は、POSTしか受け付けないURLにGETで
// 辿り着くことになるためです。書かれたものはそれと共に失われます。拒否された返信を保つのは
// このハンドラーが描くページであり、ハンドラーの手前で止まった送信には、それを戻して置く
// ページがありません。
func TestCreate_ExpiredSession(t *testing.T) {
	t.Parallel()

	const csrfToken = "test-csrf-token"

	f := newFixture(t)
	cfg := &config.Config{Env: "test"}
	auth := middleware.NewAuth(session.NewManager(repository.NewUserRepository(f.db), cfg))
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

	form := reply("セッションが切れた後に送った返信")
	form.Set("csrf_token", csrfToken)

	path := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: middleware.CSRFCookieName, Value: csrfToken})
	// トークンが名指すのは、データベースがもう持っていないセッションである。書かれた
	// ときのセッションが失効した送信が出会う状態がこれにあたる。
	req.AddCookie(&http.Cookie{Name: session.CookieName, Value: "expired-session-token"})
	req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusSeeOther)
	}
	if got, want := rec.Header().Get("Location"), templates.SignInPath().String(); got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	if got := countPosts(t, f.db, f.open); got != 1 {
		t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件", got)
	}
}

// TestCreate_ThreadFillsAfterFormIsOpenedは、上限到達前に取得したフォームと、別の
// 投稿者が上限に到達させた後の拒否を接続します。実際の投稿行を用意することで、GET・
// レス番号・最終的なDBの状態をフロー全体で一致させます。
func TestCreate_ThreadFillsAfterFormIsOpened(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	ctx := context.Background()
	// 既存投稿は作者なしにし、どちらの訪問者も連投制限中にならないようにする。
	// 一括挿入により、準備のための数百回のトランザクションを避ける。
	if _, err := f.db.Writer.ExecContext(ctx, `
		WITH RECURSIVE numbers(number) AS (
			SELECT 2 UNION ALL SELECT number + 1 FROM numbers WHERE number < ?
		)
		INSERT INTO posts (thread_id, number, body)
		SELECT ?, number, '既存の投稿' FROM numbers
	`, model.ThreadPostLimit-1, int64(f.open)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Writer.ExecContext(ctx, `
		UPDATE threads SET posts_count = ?,
			last_post_id = (SELECT id FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1),
			last_posted_at = (SELECT created_at FROM posts WHERE thread_id = ? ORDER BY number DESC LIMIT 1)
		WHERE id = ?
	`, model.ThreadPostLimit-1, int64(f.open), int64(f.open), int64(f.open)); err != nil {
		t.Fatal(err)
	}
	if got := countPosts(t, f.db, f.open); got != model.ThreadPostLimit-1 {
		t.Fatalf("準備後の投稿数 = %d、期待値 = %d", got, model.ThreadPostLimit-1)
	}

	for _, name := range []string{"alice", "bob"} {
		author := newAuthor(t, f.db, name)
		testutil.NewUserSessionBuilder(t, f.db).WithUserID(author.ID).WithToken(name + "-session").Build()
	}
	cfg := &config.Config{Env: "test", AppURL: appURL}
	threadRepo := repository.NewThreadRepository(f.db)
	boardRepo := repository.NewBoardRepository(f.db)
	categoryRepo := repository.NewCategoryRepository(f.db)
	userRepo := repository.NewUserRepository(f.db)
	threadHandler := thread.NewHandler(cfg, httperror.NewRenderer(cfg),
		usecase.NewGetCommunityNavigationUsecase(repository.NewCommunityRepository(f.db), boardRepo, repository.NewRoleRepository(f.db)),
		usecase.NewGetBoardUsecase(boardRepo, categoryRepo),
		usecase.NewGetThreadUsecase(threadRepo, boardRepo, categoryRepo,
			repository.NewPostRepository(f.db), repository.NewPostReferenceRepository(f.db), userRepo, repository.NewRoleRepository(f.db)),
		usecase.NewGetBoardThreadsUsecase(threadRepo), nil,
	)
	auth := middleware.NewAuth(session.NewManager(userRepo, cfg))
	router := chi.NewRouter()
	router.Use(middleware.PostFormLimit)
	router.Use(middleware.NewCSRF(cfg).Middleware)
	router.With(auth.SetUser).Get("/t/{id}", threadHandler.Show)
	router.With(auth.RequireAuth).Post("/t/{id}/posts", f.handler.Create)

	threadPath := templates.ThreadPath(viewmodel.ThreadID(f.open)).String()
	postPath := templates.ThreadPostsPath(viewmodel.ThreadID(f.open)).String()
	request := func(name, method, path string, form url.Values, csrf *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: session.CookieName, Value: name + "-session"})
		if csrf != nil {
			req.AddCookie(csrf)
		}
		req = req.WithContext(i18n.SetLocale(req.Context(), model.LocaleJa))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}
	openForm := func(name string) (url.Values, *http.Cookie) {
		t.Helper()
		rec := request(name, http.MethodGet, threadPath, nil, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("フォーム取得のステータスコード = %d、期待値 = 200", rec.Code)
		}
		form := replyFormValues(t, rec.Body.String(), postPath)
		for _, cookie := range rec.Result().Cookies() {
			if cookie.Name == middleware.CSRFCookieName {
				if form.Get("csrf_token") == "" || form.Get("csrf_token") != cookie.Value {
					t.Fatal("返信フォームのCSRFトークンがCookieと一致しない")
				}
				return form, cookie
			}
		}
		t.Fatal("GET応答にCSRF Cookieがない")
		return nil, nil
	}

	aliceForm, aliceCSRF := openForm("alice")
	bobForm, bobCSRF := openForm("bob")
	bobForm.Set("body", "最後の投稿")
	last := request("bob", http.MethodPost, postPath, bobForm, bobCSRF)
	if last.Code != http.StatusSeeOther {
		t.Fatalf("1000番のステータスコード = %d、期待値 = 303", last.Code)
	}
	if got, want := last.Header().Get("Location"), threadPath+"#p1000"; got != want {
		t.Errorf("Location = %q、期待値 = %q", got, want)
	}
	shown := request("bob", http.MethodGet, threadPath, nil, bobCSRF)
	if shown.Code != http.StatusOK || !strings.Contains(shown.Body.String(), `id="p1000"`) {
		t.Fatal("投稿後の閲覧ページに1000番のアンカーがない")
	}

	const keptBody = "\n書きかけの返信 <続き> & 本文\n"
	aliceForm.Set("body", keptBody)
	refused := request("alice", http.MethodPost, postPath, aliceForm, aliceCSRF)
	if refused.Code != http.StatusConflict {
		t.Fatalf("取得済みフォーム送信のステータスコード = %d、期待値 = 409", refused.Code)
	}
	markup := refused.Body.String()
	if !strings.Contains(markup, "このスレッドは投稿数の上限 (1000 件) に達しました。") {
		t.Error("上限到達の理由が表示されていない")
	}
	if strings.Contains(markup, `action="`+postPath+`"`) {
		t.Error("ロック中のスレッドへの再送フォームがある")
	}
	control := testutil.OpeningTag(t, markup, `id="body"`)
	if !strings.Contains(control, "readonly") || strings.Contains(control, "aria-invalid") {
		t.Errorf("保持された本文の属性が不正: %s", control)
	}
	root, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	var kept string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "textarea" {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					kept += child.Data
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(root)
	if kept != keptBody {
		t.Errorf("保持された本文 = %q、期待値 = %q", kept, keptBody)
	}

	direct := reply("フォームを介さない追加投稿")
	direct.Set("csrf_token", aliceCSRF.Value)
	if rec := request("alice", http.MethodPost, postPath, direct, aliceCSRF); rec.Code != http.StatusConflict {
		t.Errorf("直接POSTのステータスコード = %d、期待値 = 409", rec.Code)
	}
	if got := countPosts(t, f.db, f.open); got != model.ThreadPostLimit {
		t.Errorf("拒否後の投稿数 = %d、期待値 = %d", got, model.ThreadPostLimit)
	}
	if saved := readLatestPost(t, f.db, f.open); saved.number != model.ThreadPostLimit || saved.body != "最後の投稿" {
		t.Errorf("拒否後の最終投稿 = %+v、期待値は1000番の成功した投稿", saved)
	}
	stored, err := threadRepo.FindByID(ctx, f.open)
	if err != nil {
		t.Fatal(err)
	}
	if stored.PostsCount != model.ThreadPostLimit {
		t.Errorf("拒否後の集計 = %d、期待値 = %d", stored.PostsCount, model.ThreadPostLimit)
	}
}

// replyFormValuesは実際の返信フォームからhiddenフィールドを取り出し、サインアウト
// など他のフォームを除外します。無効な送信ボタンではフローを開始できません。
func replyFormValues(t *testing.T, markup, action string) url.Values {
	t.Helper()
	z := html.NewTokenizer(strings.NewReader(markup))
	inside := false
	values := url.Values{}
	for {
		switch z.Next() {
		case html.ErrorToken:
			t.Fatal("送信可能な返信フォームがない")
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			attrs := map[string]string{}
			for _, attr := range token.Attr {
				attrs[attr.Key] = attr.Val
			}
			if token.Data == "form" {
				inside = attrs["action"] == action && strings.EqualFold(attrs["method"], "POST")
			}
			if !inside {
				continue
			}
			if token.Data == "input" && attrs["type"] == "hidden" {
				values.Set(attrs["name"], attrs["value"])
			}
			if token.Data == "button" && attrs["type"] == "submit" {
				if _, disabled := attrs["disabled"]; disabled {
					t.Fatal("返信フォームの送信ボタンが無効")
				}
				return values
			}
		case html.EndTagToken:
			if z.Token().Data == "form" {
				inside = false
			}
		}
	}
}

// TestCreate_UnpublishedThreadは、管理者が見えない場所へ移したスレッドへの返信が、
// スレッドの取り下げを述べるページ (HTTP 404) で応答され、何も保存されないことを検証します。
// フォームは描き直しません。コミュニティがもう示していないスレッドは書き込む先ではなく、
// 送信を戻して置くページがありません。
//
// この拒否には2つの経路があり、どちらも検証します。書き込み自身が返信を拒否する経路と、
// 書き込みの前に拒否された送信 (直すところのあるもの) が、戻ってくるページのためにスレッドを
// 読み直して同じ応答に至る経路です。
func TestCreate_UnpublishedThread(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		form url.Values
	}{
		{name: "整った返信", form: reply("非公開のスレッドへの返信")},
		{name: "直すところのある返信", form: reply("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			author := newAuthor(t, f.db, "alice")

			if err := repository.NewThreadRepository(f.db).Unpublish(context.Background(), f.open); err != nil {
				t.Fatalf("Unpublish()のエラー = %v", err)
			}

			rec := httptest.NewRecorder()
			f.handler.Create(rec, newSubmitRequest(t, f.open.String(), model.LocaleJa, author, tt.form))

			if rec.Code != http.StatusNotFound {
				t.Fatalf("ステータスコード = %d、期待値 = %d", rec.Code, http.StatusNotFound)
			}
			body := rec.Body.String()
			if !strings.Contains(body, "このページは管理者により非公開にされました。") {
				t.Error("非公開のページの文言が含まれていない")
			}
			if strings.Contains(body, "ページが見つかりません") {
				t.Error("非公開のスレッドへの返信に404ページの文言が含まれている")
			}
			if got := countPosts(t, f.db, f.open); got != 1 {
				t.Errorf("スレッドの投稿 = %d 件、期待値 = 1件", got)
			}
		})
	}
}
