// Command server is the entry point for the Groobb HTTP server.
//
// [Ja] server コマンドは Groobb HTTP サーバーのエントリポイントです。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/database"
	"github.com/groobb/groobb/go/internal/dispatcher"
	"github.com/groobb/groobb/go/internal/handler/account"
	"github.com/groobb/groobb/go/internal/handler/email_confirmation"
	"github.com/groobb/groobb/go/internal/handler/health"
	"github.com/groobb/groobb/go/internal/handler/home"
	"github.com/groobb/groobb/go/internal/handler/password"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/handler/user_session"
	"github.com/groobb/groobb/go/internal/handler/welcome"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load the configuration", "error", err)
		os.Exit(1)
	}

	// Connect to PostgreSQL and verify connectivity before serving requests, so
	// a misconfigured or unreachable database fails fast at startup. The bounded
	// context only guards the initial connect/ping; the pool itself outlives it.
	//
	// [Ja] リクエストを受ける前に PostgreSQL へ接続して疎通を確認し、設定ミスや
	// 接続不能なデータベースを起動時に早期検知する。タイムアウト付き context は
	// 最初の接続/ping だけを制御し、プール自体はそれより長く生存する。
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := database.New(connectCtx, cfg.DatabaseURL)
	connectCancel()
	if err != nil {
		slog.Error("failed to connect to the database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("connected to the database")

	// Build and start the background job worker on its own pool. Sign-up is the
	// first flow to enqueue a job (the confirmation email), so the worker is
	// wired and started here; without it, enqueued jobs would never be processed.
	// The bounded context guards only pool creation; the worker runs on a
	// background context and is drained by Stop on shutdown.
	//
	// [Ja] バックグラウンドジョブのワーカーを専用プール上に構築・起動する。サインアップは
	// 最初にジョブ (確認メール) を投入するフローのため、ワーカーをここで配線・起動する。
	// これが無いと投入されたジョブは処理されない。タイムアウト付き context はプール生成
	// のみを制御し、ワーカーは background context で動き、シャットダウン時に Stop で
	// ドレインする。
	workerCtx, workerCancel := context.WithTimeout(context.Background(), 10*time.Second)
	workerClient, err := worker.NewClient(workerCtx, cfg.DatabaseURL, cfg)
	workerCancel()
	if err != nil {
		slog.Error("failed to build the worker client", "error", err)
		os.Exit(1)
	}
	if err := workerClient.Start(context.Background()); err != nil {
		slog.Error("failed to start the worker client", "error", err)
		os.Exit(1)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		if err := workerClient.Stop(stopCtx); err != nil {
			slog.Error("failed to stop the worker client", "error", err)
		}
	}()

	// Wire the request-path dependencies: repositories over the app pool, then
	// the session manager, dispatcher, validator, UseCase, and handlers.
	//
	// [Ja] リクエスト経路の依存を配線する。アプリ用プール上のリポジトリ、続いて
	// セッションマネージャ・ディスパッチャー・バリデーター・UseCase・ハンドラー。
	queries := query.New(pool)
	userRepo := repository.NewUserRepository(queries)
	userPasswordRepo := repository.NewUserPasswordRepository(queries)
	userSessionRepo := repository.NewUserSessionRepository(queries)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(queries)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(queries)

	sessionMgr := session.NewManager(userRepo, cfg)

	// Flash manager: reads and writes the one-off flash cookie. Its Middleware is
	// wired globally below so any page's layout can render a pending message.
	//
	// [Ja] フラッシュマネージャ: 一度きりのフラッシュ Cookie を読み書きする。その Middleware を
	// 下でグローバルに配線し、どのページのレイアウトからも保留中のメッセージを描画できるようにする。
	flashMgr := session.NewFlashManager(cfg)

	jobDispatcher := dispatcher.NewDispatcher(workerClient.Client())

	// One Turnstile verifier is shared across the public-form handlers (sign-up
	// here, then sign-in and password-reset). Only the secret key is needed: an
	// empty key (the disabled dev / test setup) makes Verify bypass every request,
	// and the site key is passed to templates from cfg directly.
	//
	// [Ja] Turnstile の検証器は公開フォームのハンドラー (ここではサインアップ、続いて
	// サインインとパスワードリセット) で 1 つを共有する。必要なのはシークレットキーのみ。
	// キーが空 (無効化された dev / test 構成) のとき Verify はすべてのリクエストを
	// バイパスし、サイトキーは cfg から直接テンプレートへ渡す。
	turnstileVerifier := turnstile.NewClient(cfg.TurnstileSecretKey)

	signUpValidator := validator.NewSignUpCreateValidator(userRepo)
	createSignUpUC := usecase.NewCreateSignUpUsecase(signUpValidator, emailConfirmationRepo, jobDispatcher)

	emailConfirmationValidator := validator.NewEmailConfirmationCreateValidator(emailConfirmationRepo)
	verifyEmailConfirmationUC := usecase.NewVerifyEmailConfirmationUsecase(pool, emailConfirmationValidator, emailConfirmationRepo)

	accountValidator := validator.NewAccountCreateValidator(userRepo)
	createAccountUC := usecase.NewCreateAccountUsecase(pool, accountValidator, emailConfirmationRepo, userRepo, userPasswordRepo)
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)

	signInValidator := validator.NewSignInCreateValidator(userRepo, userPasswordRepo)
	createSignInUC := usecase.NewCreateSignInUsecase(signInValidator)
	deleteSessionUC := usecase.NewDeleteSessionUsecase(userSessionRepo)

	passwordResetValidator := validator.NewPasswordResetCreateValidator()
	createPasswordResetTokenUC := usecase.NewCreatePasswordResetTokenUsecase(pool, passwordResetValidator, userRepo, passwordResetTokenRepo, jobDispatcher, cfg)

	passwordUpdateValidator := validator.NewPasswordUpdateValidator(passwordResetTokenRepo)
	updatePasswordResetUC := usecase.NewUpdatePasswordResetUsecase(pool, passwordUpdateValidator, passwordResetTokenRepo, userPasswordRepo)

	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)
	homeHandler := home.NewHandler(cfg)
	signUpHandler := sign_up.NewHandler(cfg, sessionMgr, createSignUpUC, turnstileVerifier)
	emailConfirmationHandler := email_confirmation.NewHandler(cfg, sessionMgr, verifyEmailConfirmationUC)
	accountHandler := account.NewHandler(cfg, sessionMgr, createAccountUC, createSessionUC)
	signInHandler := sign_in.NewHandler(cfg, sessionMgr, createSignInUC, createSessionUC, turnstileVerifier)
	userSessionHandler := user_session.NewHandler(sessionMgr, flashMgr, deleteSessionUC)
	passwordResetHandler := password_reset.NewHandler(cfg, createPasswordResetTokenUC, turnstileVerifier)
	passwordHandler := password.NewHandler(cfg, updatePasswordResetUC)

	authMiddleware := middleware.NewAuth(sessionMgr)
	csrf := middleware.NewCSRF(cfg)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// Resolve the request locale from Accept-Language and store it in the
	// context so handlers and templates can render localized text.
	//
	// [Ja] Accept-Language からリクエストのロケールを解決して context に格納し、
	// ハンドラーとテンプレートがローカライズされたテキストを描画できるようにする。
	r.Use(i18n.Middleware)

	// Issue and verify CSRF tokens for every route: safe requests mint the token
	// for forms to embed, and unsafe requests (the sign-up POST and later forms)
	// must echo it back.
	//
	// [Ja] 全ルートで CSRF トークンを発行・検証する。安全なリクエストはフォームが
	// 埋め込むトークンを発行し、安全でないリクエスト (サインアップ POST や後続の
	// フォーム) は同じトークンを返す必要がある。
	r.Use(csrf.Middleware)

	// Rewrite a POST carrying _method=PATCH/PUT/DELETE to that method so HTML
	// forms (which can only GET/POST) can drive the PATCH/DELETE routes (e.g. the
	// password update form posts to PATCH /password). It runs after the CSRF
	// check, which guards POST and PATCH alike, so the override does not weaken it.
	//
	// [Ja] _method=PATCH/PUT/DELETE を運ぶ POST をそのメソッドへ書き換え、(GET/POST しか
	// 送れない) HTML フォームから PATCH/DELETE ルートを動かせるようにする (例: パスワード
	// 更新フォームは PATCH /password へ POST する)。POST も PATCH も等しく守る CSRF 検証の
	// 後に走るため、オーバーライドが検証を弱めることはない。
	r.Use(middleware.MethodOverride)

	// Read any one-off flash message from its cookie into the request context and
	// clear the cookie, so a handler's redirect target renders it exactly once
	// (e.g. the sign-out success toast). It wraps every route because the shared
	// layout, which any page can render, reads the flash from the context.
	//
	// [Ja] 一度きりのフラッシュメッセージを Cookie からリクエスト context へ読み込み、Cookie を
	// 消去する。これによりハンドラーのリダイレクト先で一度だけ描画される (例: サインアウト成功の
	// toast)。フラッシュはどのページも描画しうる共通レイアウトが context から読むため、全ルートに掛ける。
	r.Use(flashMgr.Middleware)

	// Health check (no authentication required).
	//
	// [Ja] ヘルスチェック (認証不要)。
	r.Get("/health", healthHandler.Show)

	// Serve static assets (CSS / JS / images) built into ./static.
	//
	// [Ja] ./static にビルドされた静的アセット (CSS / JS / 画像) を配信する。
	fileServer := http.FileServer(http.Dir("./static"))
	r.Handle("/static/*", http.StripPrefix("/static", fileServer))

	// Top page. SetUser resolves the current user from the session cookie so the
	// handler can render by sign-in state (a signed-in visitor is redirected to
	// /home). It is scoped to this route rather than applied globally: routes that
	// never read the user (static assets, the health check) must not pay for a
	// per-request session lookup, and RequireAuth-guarded routes resolve the user
	// themselves.
	//
	// [Ja] トップページ。SetUser がセッション Cookie から現在のユーザーを解決し、ハンドラーが
	// サインイン状態で描画を出し分けられるようにする (サインイン済みの訪問者は /home へ
	// リダイレクトされる)。グローバルではなくこのルートに限定して掛ける。ユーザーを読まない
	// ルート (静的アセット・ヘルスチェック) はリクエストごとのセッション解決のコストを負う
	// べきでなく、RequireAuth で守るルートは自身でユーザーを解決するためである。
	r.With(authMiddleware.SetUser).Get("/", welcomeHandler.Show)

	// Home: the signed-in landing page. RequireAuth redirects an anonymous
	// visitor to /sign_in before the handler runs.
	//
	// [Ja] ホーム: サインイン済みの着地ページ。RequireAuth はハンドラーが走る前に
	// 匿名の訪問者を /sign_in へリダイレクトする。
	r.With(authMiddleware.RequireAuth).Get("/home", homeHandler.Show)

	// Sign-up: show the form and accept an email to issue a confirmation code.
	//
	// [Ja] サインアップ: フォームを表示し、確認コード発行のため email を受け付ける。
	r.Get("/sign_up", signUpHandler.New)
	r.Post("/sign_up", signUpHandler.Create)

	// Email confirmation: show the code-entry form and verify the code emailed
	// during sign-up.
	//
	// [Ja] メール確認: コード入力フォームを表示し、サインアップ時にメールした
	// コードを検証する。
	r.Get("/email_confirmation/new", emailConfirmationHandler.New)
	r.Post("/email_confirmation", emailConfirmationHandler.Create)

	// Account creation: show the password-setup form and create the account, then
	// sign the user in.
	//
	// [Ja] アカウント作成: パスワード設定フォームを表示してアカウントを作成し、
	// ユーザーをサインインさせる。
	r.Get("/account/new", accountHandler.New)
	r.Post("/account", accountHandler.Create)

	// Sign-in: show the form and authenticate an email and password, issuing a
	// session on success.
	//
	// [Ja] サインイン: フォームを表示し、email とパスワードを認証して、成功時に
	// セッションを発行する。
	r.Get("/sign_in", signInHandler.New)
	r.Post("/sign_in", signInHandler.Create)

	// Sign-out: delete the current session and clear the session cookie.
	//
	// [Ja] サインアウト: 現在のセッションを削除しセッション Cookie を消去する。
	r.Delete("/user_session", userSessionHandler.Delete)

	// Password reset request: show the form and accept an email to issue a reset
	// link, which is emailed to the account if one exists.
	//
	// [Ja] パスワードリセット申請: フォームを表示し、リセットリンク発行のため email を
	// 受け付ける。リンクはアカウントが存在すればそのアカウントへメールされる。
	r.Get("/password_reset/new", passwordResetHandler.New)
	r.Post("/password_reset", passwordResetHandler.Create)

	// Password reset update: show the new-password form from the emailed link and
	// set the new password, spending the reset token. The form drives PATCH via
	// the _method override.
	//
	// [Ja] パスワードリセット更新: メールのリンクから新パスワードフォームを表示し、新しい
	// パスワードを設定してリセットトークンを消費する。フォームは _method オーバーライドで
	// PATCH を動かす。
	r.Get("/password/edit", passwordHandler.Edit)
	r.Patch("/password", passwordHandler.Update)

	addr := fmt.Sprintf("0.0.0.0:%s", cfg.Port)
	slog.Info("starting the HTTP server", "addr", addr, "env", cfg.Env)

	srv := &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Graceful shutdown: on SIGINT / SIGTERM, stop accepting new connections and
	// wait for in-flight requests to finish (up to the timeout).
	//
	// [Ja] グレースフルシャットダウン。SIGINT / SIGTERM を受けたら新規接続の
	// 受け付けを止め、処理中のリクエストの完了を (タイムアウトまで) 待ちます。
	shutdownDone := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		slog.Info("received a shutdown signal")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shut down the server", "error", err)
		}
		close(shutdownDone)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start the server", "error", err)
		os.Exit(1)
	}

	// Wait for the in-flight requests to be drained before exiting. Without this,
	// main would return as soon as ListenAndServe reports ErrServerClosed, while
	// srv.Shutdown is still draining connections in the goroutine.
	//
	// [Ja] 終了する前に処理中リクエストのドレイン完了を待つ。これが無いと、
	// goroutine 内の srv.Shutdown がまだ接続をドレインしている最中でも、
	// ListenAndServe が ErrServerClosed を返した時点で main が返ってしまう。
	<-shutdownDone
	slog.Info("the server has stopped")
}
