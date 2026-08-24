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
	"github.com/groobb/groobb/go/internal/handler/settings"
	"github.com/groobb/groobb/go/internal/handler/settings_email"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/handler/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/handler/user_session"
	"github.com/groobb/groobb/go/internal/handler/welcome"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/worker"
	"github.com/groobb/groobb/go/static"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load the configuration", "error", err)
		os.Exit(1)
	}

	// Open the SQLite database and verify connectivity before serving requests, so
	// a misconfigured or unopenable database fails fast at startup. The bounded
	// context only guards the initial open/ping; the pools themselves outlive it.
	//
	// [Ja] リクエストを受ける前に SQLite データベースを開いて疎通を確認し、設定ミスや
	// 開けないデータベースを起動時に早期検知する。タイムアウト付き context は
	// 最初のオープン / ping だけを制御し、プール自体はそれより長く生存する。
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	db, err := database.Open(connectCtx, cfg.DatabasePath)
	connectCancel()
	if err != nil {
		slog.Error("failed to open the database", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("failed to close the database", "error", err)
		}
	}()
	slog.Info("opened the database")

	// Bring the schema up to date on startup. A self-hosted instance is expected
	// to be upgraded by replacing the binary and restarting it, so applying the
	// migrations here is what keeps the database in step with the code without the
	// operator running a separate command.
	//
	// [Ja] 起動時にスキーマを最新へ揃える。セルフホストのインスタンスはバイナリを置き換えて
	// 再起動することで更新される想定のため、ここでマイグレーションを適用することが、運用者に
	// 別のコマンドを求めずにデータベースをコードへ追随させる手段になる。
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 60*time.Second)
	err = database.Migrate(migrateCtx, db.Writer)
	migrateCancel()
	if err != nil {
		slog.Error("failed to migrate the database", "error", err)
		os.Exit(1)
	}

	// Build and start the background job worker on its own connection. Sign-up is the
	// first flow to enqueue a job (the confirmation email), so the worker is
	// wired and started here; without it, enqueued jobs would never be processed.
	// The bounded context guards only opening that connection; the worker runs on a
	// background context and is drained by Stop on shutdown.
	//
	// [Ja] バックグラウンドジョブのワーカーを専用の接続上に構築・起動する。サインアップは
	// 最初にジョブ (確認メール) を投入するフローのため、ワーカーをここで配線・起動する。
	// これが無いと投入されたジョブは処理されない。タイムアウト付き context はその接続を開く
	// 処理のみを制御し、ワーカーは background context で動き、シャットダウン時に Stop で
	// ドレインする。
	workerCtx, workerCancel := context.WithTimeout(context.Background(), 10*time.Second)
	workerClient, err := worker.NewClient(workerCtx, cfg.DatabasePath, cfg)
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

	// Wire the request-path dependencies: repositories over the application's
	// connection, then the session manager, dispatcher, validator, UseCase, and
	// handlers.
	//
	// [Ja] リクエスト経路の依存を配線する。アプリ用の接続上のリポジトリ、続いて
	// セッションマネージャ・ディスパッチャー・バリデーター・UseCase・ハンドラー。
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)

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
	verifyEmailConfirmationUC := usecase.NewVerifyEmailConfirmationUsecase(db.Writer, emailConfirmationValidator, emailConfirmationRepo)

	accountValidator := validator.NewAccountCreateValidator(userRepo)
	createAccountUC := usecase.NewCreateAccountUsecase(db.Writer, accountValidator, emailConfirmationRepo, userRepo, userPasswordRepo)
	createSessionUC := usecase.NewCreateSessionUsecase(userSessionRepo)

	signInValidator := validator.NewSignInCreateValidator(userRepo, userPasswordRepo, userTwoFactorAuthRepo)
	createSignInUC := usecase.NewCreateSignInUsecase(signInValidator)
	deleteSessionUC := usecase.NewDeleteSessionUsecase(userSessionRepo)

	signInTwoFactorValidator := validator.NewSignInTwoFactorCreateValidator(userTwoFactorAuthRepo)
	createSignInTwoFactorUC := usecase.NewCreateSignInTwoFactorUsecase(signInTwoFactorValidator)

	signInTwoFactorRecoveryValidator := validator.NewSignInTwoFactorRecoveryCreateValidator(userTwoFactorAuthRepo)
	createSignInTwoFactorRecoveryUC := usecase.NewCreateSignInTwoFactorRecoveryUsecase(db.Writer, signInTwoFactorRecoveryValidator, userTwoFactorAuthRepo, userSessionRepo)

	passwordResetValidator := validator.NewPasswordResetCreateValidator()
	createPasswordResetTokenUC := usecase.NewCreatePasswordResetTokenUsecase(db.Writer, passwordResetValidator, userRepo, passwordResetTokenRepo, jobDispatcher, cfg)

	passwordUpdateValidator := validator.NewPasswordUpdateValidator(passwordResetTokenRepo)
	updatePasswordResetUC := usecase.NewUpdatePasswordResetUsecase(db.Writer, passwordUpdateValidator, passwordResetTokenRepo, userPasswordRepo)

	settingsEmailUpdateValidator := validator.NewSettingsEmailUpdateValidator(userRepo, userPasswordRepo)
	createEmailChangeUC := usecase.NewCreateEmailChangeUsecase(db.Writer, settingsEmailUpdateValidator, emailConfirmationRepo, jobDispatcher)

	settingsEmailConfirmationValidator := validator.NewSettingsEmailConfirmationCreateValidator(emailConfirmationRepo)
	verifyEmailChangeUC := usecase.NewVerifyEmailChangeUsecase(db.Writer, settingsEmailConfirmationValidator, emailConfirmationRepo, userRepo, jobDispatcher)

	settingsWithdrawalDeleteValidator := validator.NewSettingsWithdrawalDeleteValidator(userPasswordRepo)
	deleteAccountUC := usecase.NewDeleteAccountUsecase(db.Writer, settingsWithdrawalDeleteValidator, userRepo, userSessionRepo)

	settingsTwoFactorAuthCreateValidator := validator.NewSettingsTwoFactorAuthCreateValidator(userTwoFactorAuthRepo)
	settingsTwoFactorAuthDeleteValidator := validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, userTwoFactorAuthRepo)
	prepareTwoFactorAuthUC := usecase.NewPrepareTwoFactorAuthUsecase(userTwoFactorAuthRepo)
	enableTwoFactorAuthUC := usecase.NewEnableTwoFactorAuthUsecase(settingsTwoFactorAuthCreateValidator, userTwoFactorAuthRepo)
	disableTwoFactorAuthUC := usecase.NewDisableTwoFactorAuthUsecase(settingsTwoFactorAuthDeleteValidator, userTwoFactorAuthRepo)

	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)
	homeHandler := home.NewHandler(cfg)
	signUpHandler := sign_up.NewHandler(cfg, sessionMgr, createSignUpUC, turnstileVerifier)
	emailConfirmationHandler := email_confirmation.NewHandler(cfg, sessionMgr, verifyEmailConfirmationUC)
	accountHandler := account.NewHandler(cfg, sessionMgr, createAccountUC, createSessionUC)
	signInHandler := sign_in.NewHandler(cfg, sessionMgr, createSignInUC, createSessionUC, turnstileVerifier)
	signInTwoFactorHandler := sign_in_two_factor.NewHandler(cfg, sessionMgr, createSignInTwoFactorUC, createSessionUC)
	signInTwoFactorRecoveryHandler := sign_in_two_factor_recovery.NewHandler(cfg, sessionMgr, createSignInTwoFactorRecoveryUC)
	userSessionHandler := user_session.NewHandler(sessionMgr, flashMgr, deleteSessionUC)
	passwordResetHandler := password_reset.NewHandler(cfg, createPasswordResetTokenUC, turnstileVerifier)
	passwordHandler := password.NewHandler(cfg, updatePasswordResetUC)
	settingsHandler := settings.NewHandler(cfg)
	settingsEmailHandler := settings_email.NewHandler(cfg, createEmailChangeUC)
	settingsEmailConfirmationHandler := settings_email_confirmation.NewHandler(cfg, flashMgr, verifyEmailChangeUC)
	settingsTwoFactorAuthHandler := settings_two_factor_auth.NewHandler(cfg, flashMgr, prepareTwoFactorAuthUC, enableTwoFactorAuthUC, disableTwoFactorAuthUC)
	settingsWithdrawalHandler := settings_withdrawal.NewHandler(cfg, sessionMgr, flashMgr, deleteAccountUC)

	authMiddleware := middleware.NewAuth(sessionMgr)
	csrf := middleware.NewCSRF(cfg)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// Send a URL carrying a trailing slash on to the same URL without one, so a
	// page answers from a single address instead of two that hold the same
	// content. It runs ahead of the middlewares below because a request that ends
	// here never reaches a handler: resolving a locale or minting a CSRF token
	// would be wasted, and the flash middleware would consume the one-off message
	// the visitor is on their way to read.
	//
	// [Ja] 末尾スラッシュ付きの URL を、スラッシュ無しの同じ URL へ送る。ページが
	// 同じ内容を持つ 2 つのアドレスではなく 1 つのアドレスから応答するようにするため
	// である。下のミドルウェアより前に走らせるのは、ここで終わるリクエストがハンドラーに
	// 到達しないためである。ロケールの解決も CSRF トークンの発行も無駄になり、フラッシュの
	// ミドルウェアは訪問者がこれから読む一度きりのメッセージを消費してしまう。
	r.Use(chimiddleware.RedirectSlashes)

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

	// Serve the static assets (CSS / JS) from the copy embedded in the binary, so
	// that the server finds them wherever it is started from rather than only
	// alongside a ./static directory. AssetCache declares how long a browser may
	// keep them; the URLs carry the asset version, so a deploy hands out new ones.
	//
	// [Ja] 静的アセット (CSS / JS) はバイナリに埋め込まれた複製から配信する。./static
	// ディレクトリの隣でなくとも、どこで起動してもサーバーがアセットを見つけられるように
	// するためである。AssetCache はブラウザが保持してよい期間を宣言する。URL は
	// アセットバージョンを伴うため、デプロイのたびに新しい URL が配られる。
	fileServer := http.FileServer(http.FS(static.Assets()))
	r.With(middleware.AssetCache(cfg)).Handle("/static/*", http.StripPrefix("/static", fileServer))

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

	// Sign-in two-factor challenge: show the TOTP code-entry form and verify the
	// code to finish signing in a 2FA-enabled account, issuing the session on
	// success. These are public routes reached mid-sign-in; the pending user is
	// resolved from the short-lived two-factor cookie set by the password step, not
	// a session.
	//
	// [Ja] サインインの 2 段階認証チャレンジ: TOTP コード入力フォームを表示し、コードを検証して
	// 2FA 有効なアカウントのサインインを完了させ、成功時にセッションを発行する。これらは
	// サインインの途中で通る公開ルートで、保留中ユーザーはセッションではなくパスワードのステップが
	// 設定した短命の 2 段階認証 Cookie から解決する。
	r.Get("/sign_in/two_factor/new", signInTwoFactorHandler.New)
	r.Post("/sign_in/two_factor", signInTwoFactorHandler.Create)

	// Sign-in two-factor recovery-code challenge: show the recovery-code entry form
	// and verify a code to finish signing in when the authenticator app is
	// unavailable, consuming the one-time code and issuing the session on success.
	// Like the TOTP challenge, these are public routes reached mid-sign-in; the
	// pending user is resolved from the short-lived two-factor cookie, not a session.
	//
	// [Ja] サインインの 2 段階認証リカバリーコードチャレンジ: 認証アプリを使えないときに
	// リカバリーコード入力フォームを表示し、コードを検証してサインインを完了させ、成功時に
	// 1 回使い切りのコードを消費してセッションを発行する。TOTP チャレンジと同様、これらは
	// サインインの途中で通る公開ルートで、保留中ユーザーはセッションではなく短命の 2 段階認証
	// Cookie から解決する。
	r.Get("/sign_in/two_factor/recovery/new", signInTwoFactorRecoveryHandler.New)
	r.Post("/sign_in/two_factor/recovery", signInTwoFactorRecoveryHandler.Create)

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

	// Settings hub: the landing page that links to the individual settings screens
	// (email change for now). It is behind RequireAuth.
	//
	// [Ja] 設定ハブ: 各設定画面 (今はメールアドレス変更) へリンクする着地ページ。
	// RequireAuth の背後に置く。
	r.With(authMiddleware.RequireAuth).Get("/settings", settingsHandler.Show)

	// Settings — email change: show the change form (with the current address) and
	// accept a new email plus the current password to issue a confirmation code.
	// Both are behind RequireAuth; the form drives PATCH via the _method override.
	//
	// [Ja] 設定 — メールアドレス変更: 変更フォーム (現在のアドレス付き) を表示し、新しい
	// email と現在のパスワードを受け付けて確認コードを発行する。どちらも RequireAuth の
	// 背後に置き、フォームは _method オーバーライドで PATCH を動かす。
	r.With(authMiddleware.RequireAuth).Get("/settings/email/edit", settingsEmailHandler.Edit)
	r.With(authMiddleware.RequireAuth).Patch("/settings/email", settingsEmailHandler.Update)

	// Settings — email change confirmation: show the code-entry form and verify the
	// code emailed to the new address, which applies the change on success. Both are
	// behind RequireAuth; the pending confirmation is resolved from the signed-in
	// user, not a handoff cookie.
	//
	// [Ja] 設定 — メールアドレス変更の確認: コード入力フォームを表示し、新しいアドレスに
	// メールしたコードを検証する。成功時に変更を適用する。どちらも RequireAuth の背後に置き、
	// 保留中の確認は受け渡し Cookie ではなくサインイン済みユーザーから解決する。
	r.With(authMiddleware.RequireAuth).Get("/settings/email/confirmation/new", settingsEmailConfirmationHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/settings/email/confirmation", settingsEmailConfirmationHandler.Create)

	// Settings — two-factor authentication: show the enrollment form (QR code and
	// manual-entry key) and enable 2FA after the user confirms a TOTP code, which
	// activates the setting and shows the one-time recovery codes. Both are behind
	// RequireAuth; the setup (GET) shows the enrollment form when 2FA is off or the
	// disable confirmation form when it is on, the enable (POST) is a plain POST (no
	// method override), and the disable (DELETE) is reached from the disable form via
	// the _method override.
	//
	// [Ja] 設定 — 2 段階認証: 2FA が無効なら登録フォーム (QR コードと手動入力キー) を、有効なら
	// 無効化の確認フォームを表示し、ユーザーが TOTP コードを確認した後に 2FA を有効化する。
	// 有効化は設定をアクティブにし、1 回使い切りのリカバリーコードを表示する。無効化は再認証
	// (現在のパスワードか現在の TOTP コード) の後に設定を削除する。すべて RequireAuth の背後に
	// 置き、設定 (GET) は登録 / 無効化フォームを、有効化 (POST) は素の POST、無効化 (DELETE) は
	// 無効化フォームから _method オーバーライドで到達する。
	r.With(authMiddleware.RequireAuth).Get("/settings/two_factor_auth/new", settingsTwoFactorAuthHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/settings/two_factor_auth", settingsTwoFactorAuthHandler.Create)
	r.With(authMiddleware.RequireAuth).Delete("/settings/two_factor_auth", settingsTwoFactorAuthHandler.Delete)

	// Settings — account withdrawal: show the confirmation form (with the current-
	// password field) and execute the withdrawal, which soft-deletes and anonymizes
	// the account and deletes all of its sessions. Both are behind RequireAuth; the
	// form drives DELETE via the _method override. The settings hub does not link
	// here yet (added in a later task), so the page is reached only by direct URL.
	//
	// [Ja] 設定 — 退会: 確認フォーム (現在のパスワードフィールド付き) を表示し、退会を実行する。
	// 退会の実行はアカウントを論理削除・匿名化し、その全セッションを削除する。どちらも
	// RequireAuth の背後に置き、フォームは _method オーバーライドで DELETE を動かす。設定ハブ
	// からのリンクはまだ無い (後続タスクで追加) ため、このページは URL 直打ちでのみ到達する。
	r.With(authMiddleware.RequireAuth).Get("/settings/withdrawal/new", settingsWithdrawalHandler.New)
	r.With(authMiddleware.RequireAuth).Delete("/settings/withdrawal", settingsWithdrawalHandler.Delete)

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
