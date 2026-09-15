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
	"github.com/groobb/groobb/go/internal/handler/admin"
	"github.com/groobb/groobb/go/internal/handler/admin_user"
	"github.com/groobb/groobb/go/internal/handler/admin_user_role"
	"github.com/groobb/groobb/go/internal/handler/board"
	"github.com/groobb/groobb/go/internal/handler/category"
	"github.com/groobb/groobb/go/internal/handler/email_confirmation"
	"github.com/groobb/groobb/go/internal/handler/health"
	"github.com/groobb/groobb/go/internal/handler/home"
	"github.com/groobb/groobb/go/internal/handler/password"
	"github.com/groobb/groobb/go/internal/handler/password_reset"
	"github.com/groobb/groobb/go/internal/handler/post"
	"github.com/groobb/groobb/go/internal/handler/post_unpublication"
	"github.com/groobb/groobb/go/internal/handler/settings"
	"github.com/groobb/groobb/go/internal/handler/settings_email"
	"github.com/groobb/groobb/go/internal/handler/settings_email_confirmation"
	"github.com/groobb/groobb/go/internal/handler/settings_two_factor_auth"
	"github.com/groobb/groobb/go/internal/handler/settings_withdrawal"
	"github.com/groobb/groobb/go/internal/handler/sign_in"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor"
	"github.com/groobb/groobb/go/internal/handler/sign_in_two_factor_recovery"
	"github.com/groobb/groobb/go/internal/handler/sign_up"
	"github.com/groobb/groobb/go/internal/handler/thread"
	"github.com/groobb/groobb/go/internal/handler/thread_lock"
	"github.com/groobb/groobb/go/internal/handler/thread_unpublication"
	"github.com/groobb/groobb/go/internal/handler/user_session"
	"github.com/groobb/groobb/go/internal/handler/welcome"
	"github.com/groobb/groobb/go/internal/httperror"
	"github.com/groobb/groobb/go/internal/i18n"
	"github.com/groobb/groobb/go/internal/middleware"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/templates"
	"github.com/groobb/groobb/go/internal/turnstile"
	"github.com/groobb/groobb/go/internal/usecase"
	"github.com/groobb/groobb/go/internal/validator"
	"github.com/groobb/groobb/go/internal/viewmodel"
	"github.com/groobb/groobb/go/internal/worker"
	"github.com/groobb/groobb/go/static"
)

// registerAdminRoutesは、コミュニティの管理画面と、そこから送信されるロールの
// 書き込みを登録します。どれもRequireAuthの背後にあり、その登録を1つの関数に持つ
// ことが、サーバー自身が配信するルートをテストから動かせるようにしています。
func registerAdminRoutes(
	r chi.Router,
	auth *middleware.Auth,
	hub *admin.Handler,
	users *admin_user.Handler,
	userRoles *admin_user_role.Handler,
) {
	// 管理ハブ: コミュニティの管理画面 (今は利用者一覧) へリンクする着地ページ。
	// RequireAuthの背後に置き、匿名の訪問者はサインインへ追い返される。サインイン済みの
	// 訪問者がこれを開いてよいかどうかは、ハンドラーの背後のUseCaseが決め、拒否には
	// 403ページで応答する。
	r.With(auth.RequireAuth).Get("/admin", hub.Show)

	// 管理 — 利用者一覧: コミュニティのアカウントの1ページを、atnameの先頭部分で
	// 絞り込み、クエリ文字列でページを送って読む。上のハブと同じくRequireAuthの背後に
	// 置く。サインイン済みの訪問者がこれを読んでよいかどうかはハンドラーの背後のUseCaseが
	// 決め、ハンドラーは拒否には403ページで応答する。整数でないページ番号と最初のページ
	// より前の番号には404ページで、最後のページより後ろの番号には戻る道を持つ空の一覧で
	// 応答する。
	r.With(auth.RequireAuth).Get("/admin/users", users.Index)

	// 管理 - 利用者のロール: アカウントにロールを与え、また取り上げる。どちらも、
	// ボタンがここへ送信する一覧と同じくRequireAuthの背後に置く。サインイン済みの訪問者が
	// ロールを配ってよいかどうかはそれぞれの背後のUseCaseが決め、ハンドラーは拒否には
	// 403ページで、存在しないアカウントやロールには404ページで応答する。付与は素のPOSTで
	// ロールを送信が名指し、剥奪は割当をアドレスが名指して、一覧のフォームから _method
	// オーバーライドで到達する。コミュニティを管理者のいない状態にすることは拒否し、その拒否は
	// フラッシュとして一覧に戻ってくる。
	r.With(auth.RequireAuth).Post("/admin/users/{id}/roles", userRoles.Create)
	r.With(auth.RequireAuth).Delete("/admin/users/{id}/roles/{name}", userRoles.Delete)
}

// runServeはHTTPサーバーを起動し、シャットダウンが完了するまでブロックします。
func runServe() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load the configuration", "error", err)
		os.Exit(1)
	}

	// リクエストを受ける前にSQLiteデータベースを開いて疎通を確認し、設定ミスや
	// 開けないデータベースを起動時に早期検知する。タイムアウト付きcontextは
	// 最初のオープン / pingだけを制御し、プール自体はそれより長く生存する。
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

	// 起動時にスキーマを最新へ揃える。セルフホストのインスタンスはバイナリを置き換えて
	// 再起動することで更新される想定のため、ここでマイグレーションを適用することが、運用者に
	// 別のコマンドを求めずにデータベースをコードへ追随させる手段になる。
	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 60*time.Second)
	err = database.Migrate(migrateCtx, db.Writer)
	migrateCancel()
	if err != nil {
		slog.Error("failed to migrate the database", "error", err)
		os.Exit(1)
	}

	// バックグラウンドジョブのワーカーを専用の接続上に構築・起動する。サインアップは
	// 最初にジョブ (確認メール) を投入するフローのため、ワーカーをここで配線・起動する。
	// これが無いと投入されたジョブは処理されない。タイムアウト付きcontextはその接続を開く
	// 処理のみを制御し、ワーカーはbackground contextで動き、シャットダウン時にStopで
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

	// リクエスト経路の依存を配線する。アプリ用の接続上のリポジトリ、続いて
	// セッションマネージャ・ディスパッチャー・バリデーター・UseCase・ハンドラー。
	userRepo := repository.NewUserRepository(db)
	userPasswordRepo := repository.NewUserPasswordRepository(db)
	userSessionRepo := repository.NewUserSessionRepository(db)
	emailConfirmationRepo := repository.NewEmailConfirmationRepository(db)
	passwordResetTokenRepo := repository.NewPasswordResetTokenRepository(db)
	userTwoFactorAuthRepo := repository.NewUserTwoFactorAuthRepository(db)
	communityRepo := repository.NewCommunityRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	boardRepo := repository.NewBoardRepository(db)
	threadRepo := repository.NewThreadRepository(db)
	postRepo := repository.NewPostRepository(db)
	postReferenceRepo := repository.NewPostReferenceRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRoleRepo := repository.NewUserRoleRepository(db)
	moderationLogRepo := repository.NewModerationLogRepository(db)

	sessionMgr := session.NewManager(userRepo, cfg)

	// フラッシュマネージャ: 一度きりのフラッシュCookieを読み書きする。そのMiddlewareを
	// 下でグローバルに配線し、どのページのレイアウトからも保留中のメッセージを描画できるようにする。
	flashMgr := session.NewFlashManager(cfg)

	jobDispatcher := dispatcher.NewDispatcher(workerClient.Client())

	// Turnstileの検証器は公開フォームのハンドラー (ここではサインアップ、続いて
	// サインインとパスワードリセット) で1つを共有する。必要なのはシークレットキーのみ。
	// キーが空 (無効化されたdev / test構成) のときVerifyはすべてのリクエストを
	// バイパスし、サイトキーはcfgから直接テンプレートへ渡す。
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
	deleteAccountUC := usecase.NewDeleteAccountUsecase(db.Writer, settingsWithdrawalDeleteValidator, userRepo, userSessionRepo, roleRepo, userRoleRepo)

	settingsTwoFactorAuthCreateValidator := validator.NewSettingsTwoFactorAuthCreateValidator(userTwoFactorAuthRepo)
	settingsTwoFactorAuthDeleteValidator := validator.NewSettingsTwoFactorAuthDeleteValidator(userPasswordRepo, userTwoFactorAuthRepo)
	prepareTwoFactorAuthUC := usecase.NewPrepareTwoFactorAuthUsecase(userTwoFactorAuthRepo)
	enableTwoFactorAuthUC := usecase.NewEnableTwoFactorAuthUsecase(settingsTwoFactorAuthCreateValidator, userTwoFactorAuthRepo)
	disableTwoFactorAuthUC := usecase.NewDisableTwoFactorAuthUsecase(settingsTwoFactorAuthDeleteValidator, userTwoFactorAuthRepo)

	getCommunityUC := usecase.NewGetCommunityUsecase(communityRepo)
	getCommunityNavigationUC := usecase.NewGetCommunityNavigationUsecase(communityRepo, boardRepo, roleRepo)
	getCommunityHomeUC := usecase.NewGetCommunityHomeUsecase(boardRepo, threadRepo)
	getCategoryUC := usecase.NewGetCategoryUsecase(categoryRepo)
	getCategoryBoardsUC := usecase.NewGetCategoryBoardsUsecase(boardRepo)
	getBoardUC := usecase.NewGetBoardUsecase(boardRepo, categoryRepo)
	getBoardThreadsUC := usecase.NewGetBoardThreadsUsecase(threadRepo)
	getThreadUC := usecase.NewGetThreadUsecase(threadRepo, boardRepo, categoryRepo, postRepo, postReferenceRepo, userRepo, roleRepo)
	getThreadSummaryUC := usecase.NewGetThreadSummaryUsecase(threadRepo)

	threadCreateValidator := validator.NewThreadCreateValidator()
	createThreadUC := usecase.NewCreateThreadUsecase(db.Writer, threadCreateValidator, boardRepo, threadRepo, postRepo, userRepo)

	postCreateValidator := validator.NewPostCreateValidator()
	createPostUC := usecase.NewCreatePostUsecase(db.Writer, postCreateValidator, threadRepo, postRepo, postReferenceRepo, userRepo)

	getAdminHomeUC := usecase.NewGetAdminHomeUsecase(roleRepo)
	getAdminUsersUC := usecase.NewGetAdminUsersUsecase(roleRepo, userRepo)
	grantUserRoleUC := usecase.NewGrantUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo)
	revokeUserRoleUC := usecase.NewRevokeUserRoleUsecase(db.Writer, roleRepo, userRepo, userRoleRepo)

	moderationLogCreateValidator := validator.NewModerationLogCreateValidator()
	getThreadModerationUC := usecase.NewGetThreadModerationUsecase(roleRepo, threadRepo, postRepo, userRepo)
	lockThreadUC := usecase.NewLockThreadUsecase(db.Writer, moderationLogCreateValidator, roleRepo, threadRepo, moderationLogRepo)
	unlockThreadUC := usecase.NewUnlockThreadUsecase(db.Writer, roleRepo, threadRepo, moderationLogRepo)
	unpublishThreadUC := usecase.NewUnpublishThreadUsecase(db.Writer, moderationLogCreateValidator, roleRepo, threadRepo, boardRepo, moderationLogRepo)
	unpublishPostUC := usecase.NewUnpublishPostUsecase(db.Writer, moderationLogCreateValidator, roleRepo, threadRepo, postRepo, moderationLogRepo)

	errorRenderer := httperror.NewRenderer(cfg)

	healthHandler := health.NewHandler()
	welcomeHandler := welcome.NewHandler(cfg)
	homeHandler := home.NewHandler(cfg, getCommunityNavigationUC, getCommunityHomeUC)
	categoryHandler := category.NewHandler(cfg, errorRenderer, getCommunityNavigationUC, getCategoryUC, getCategoryBoardsUC)
	boardHandler := board.NewHandler(cfg, errorRenderer, getCommunityNavigationUC, getBoardUC, getBoardThreadsUC)
	threadHandler := thread.NewHandler(cfg, errorRenderer, getCommunityNavigationUC, getBoardUC, getThreadUC, getBoardThreadsUC, createThreadUC)
	postHandler := post.NewHandler(cfg, errorRenderer, getCommunityNavigationUC, getThreadSummaryUC, createPostUC)
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
	threadLockHandler := thread_lock.NewHandler(cfg, errorRenderer, flashMgr, getThreadModerationUC, lockThreadUC, unlockThreadUC)
	threadUnpublicationHandler := thread_unpublication.NewHandler(cfg, errorRenderer, flashMgr, getThreadModerationUC, unpublishThreadUC)
	postUnpublicationHandler := post_unpublication.NewHandler(cfg, errorRenderer, flashMgr, getThreadModerationUC, unpublishPostUC)
	adminHandler := admin.NewHandler(cfg, errorRenderer, getAdminHomeUC)
	adminUserHandler := admin_user.NewHandler(cfg, errorRenderer, getAdminUsersUC)
	adminUserRoleHandler := admin_user_role.NewHandler(errorRenderer, flashMgr, grantUserRoleUC, revokeUserRoleUC)

	authMiddleware := middleware.NewAuth(sessionMgr)
	siteName := middleware.NewSiteName(
		func(ctx context.Context) (string, error) {
			output, err := getCommunityUC.Execute(ctx)
			if err != nil {
				return "", err
			}
			if output.Community == nil {
				return "", nil
			}
			return output.Community.Name, nil
		},
		viewmodel.SetSiteName,
	)
	csrf := middleware.NewCSRF(cfg)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.Recoverer)

	// アプリケーションが書き出すすべてのリダイレクトの発行元としてGroobbを示す。
	// 意図せず転送されるURLを、前段のCloudflareとプロキシの設定を読むのではなく、
	// ヘッダー1つを読んでどの層のものか辿れるようにするためである。末尾スラッシュの正規化
	// より上に登録するのは、その正規化が自身でリクエストに応答するためであり、ミドルウェアが
	// 到達できるのはそれより下のミドルウェアが書いたレスポンスだけである。
	r.Use(middleware.RedirectBy)

	// より具体的なハンドラーやミドルウェアが方針を選んでいないHTMLレスポンスに、
	// privateな再検証ポリシーを与える。下のルートは訪問者固有のCSRFトークンを発行して
	// 埋め込むため、ブラウザには保存を許可しつつ共有キャッシュには保存させない。末尾
	// スラッシュのリダイレクトも包み、アセット・機密・404のレスポンスはそれぞれの方針を
	// 維持する。
	r.Use(middleware.HTMLCache)

	// 末尾スラッシュ付きのURLを、スラッシュ無しの同じURLへ送る。ページが
	// 同じ内容を持つ2つのアドレスではなく1つのアドレスから応答するようにするため
	// である。下のミドルウェアより前に走らせるのは、ここで終わるリクエストがハンドラーに
	// 到達しないためである。ロケールの解決もCSRFトークンの発行も無駄になり、フラッシュの
	// ミドルウェアは訪問者がこれから読む一度きりのメッセージを消費してしまう。
	r.Use(chimiddleware.RedirectSlashes)

	// 投稿を送信する2つのPOSTルートのボディを、他の何かがそれを読む前に制限して
	// 解析する。下のCSRF検証は送信されたトークンをフォームから読み、その読み取りがボディを
	// 解析する箇所である。そこに先に到達すると、大きすぎる・デコードできないボディはトークンが
	// 無い状態と見分けられず、失敗した読み取りが残す空のフォームは、ハンドラーに中身の無い
	// 送信として受け取られてしまう。その手前でなら、そうしたリクエストをそれとして応答できる。
	//
	// 下のミドルウェアより上のここに登録するのは、上の末尾スラッシュのリダイレクトと同じ理由に
	// よる。ここで追い返したリクエストはハンドラーに到達しないため、それらが解決するロケールも、
	// サイトを名指すために読むコミュニティも、どちらも運ばない応答のための仕事になる。
	r.Use(middleware.PostFormLimit)

	// Accept-Languageからリクエストのロケールを解決してcontextに格納し、
	// ハンドラーとテンプレートがローカライズされたテキストを描画できるようにする。
	r.Use(i18n.Middleware)

	// リクエストパスをcontextに格納し、共通レイアウトのナビゲーションが今描画して
	// いるページを指すリンクにaria-current="page" を付けられるようにする。どのページも
	// 描画しうるレイアウトがcontextからパスを読むため、全ルートに掛ける。
	r.Use(templates.CurrentPathMiddleware)

	// このインスタンスが運営するコミュニティの名前をcontextに格納し、今描画して
	// いるページのタイトルがそれで終われるようにする。どのページを描画してもサイトは同じ
	// 1つであり、コミュニティのシェルの外のページ (サインインフォーム・404) は名前を
	// 取り出せるコミュニティを自前では読み込まないため、全ルートに掛ける。
	r.Use(siteName.Middleware)

	// 全ルートでCSRFトークンを発行・検証する。安全なリクエストはフォームが
	// 埋め込むトークンを発行し、安全でないリクエスト (サインアップPOSTや後続の
	// フォーム) は同じトークンを返す必要がある。
	r.Use(csrf.Middleware)

	// _method=PATCH/PUT/DELETEを運ぶPOSTをそのメソッドへ書き換え、(GET/POSTしか
	// 送れない) HTMLフォームからPATCH/DELETEルートを動かせるようにする (例: パスワード
	// 更新フォームはPATCH /passwordへPOSTする)。POSTもPATCHも等しく守るCSRF検証の
	// 後に走るため、オーバーライドが検証を弱めることはない。
	r.Use(middleware.MethodOverride)

	// 一度きりのフラッシュメッセージをCookieからリクエストcontextへ読み込み、Cookieを
	// 消去する。これによりハンドラーのリダイレクト先で一度だけ描画される (例: サインアウト成功の
	// toast)。フラッシュはどのページも描画しうる共通レイアウトがcontextから読むため、全ルートに掛ける。
	r.Use(flashMgr.Middleware)

	// どのルートにも一致しないリクエストには、chiの既定の平文1行ではなく共通の
	// 404ページで応答する。ルーターを包むのではなくルーターに登録するため、上のミドル
	// ウェアは変わらず走る。ページは訪問者の言語で描画するのに、それらが解決するロケールを
	// 必要とする。
	r.NotFound(errorRenderer.NotFound)

	// ヘルスチェック (認証不要)。
	r.Get("/health", healthHandler.Show)

	// 静的アセット (CSS / JS) はバイナリに埋め込まれた複製から配信する。./static
	// ディレクトリの隣でなくとも、どこで起動してもサーバーがアセットを見つけられるように
	// するためである。AssetCacheはブラウザが保持してよい期間を宣言する。URLは
	// アセットバージョンを伴うため、デプロイのたびに新しいURLが配られる。
	fileServer := http.FileServer(http.FS(static.Assets()))
	r.With(middleware.AssetCache(cfg)).Handle("/static/*", http.StripPrefix("/static", fileServer))

	// トップページ。SetUserがセッションCookieから現在のユーザーを解決し、ハンドラーが
	// サインイン状態で描画を出し分けられるようにする (サインイン済みの訪問者は /homeへ
	// リダイレクトされる)。グローバルではなくこのルートに限定して掛ける。ユーザーを読まない
	// ルート (静的アセット・ヘルスチェック) はリクエストごとのセッション解決のコストを負う
	// べきでなく、RequireAuthで守るルートは自身でユーザーを解決するためである。
	r.With(authMiddleware.SetUser).Get("/", welcomeHandler.Show)

	// ホーム: サインイン済みの着地ページ。RequireAuthはハンドラーが走る前に
	// 匿名の訪問者を /sign_inへリダイレクトする。
	r.With(authMiddleware.RequireAuth).Get("/home", homeHandler.Show)

	// カテゴリー: そのカテゴリーがまとめる掲示板。サインアウト状態でも読めるため、
	// RequireAuthがサインインを要求するのではなくSetUserが訪問者を解決する。コミュニティの
	// ページは公開であり、サイドバーはその背後にアカウントがあるときだけアカウント操作を
	// 描画する。ミドルウェアをグローバルに掛けずこのルートに限定するのは、トップページと
	// 同じ理由である。
	r.With(authMiddleware.SetUser).Get("/c/{slug}", categoryHandler.Show)

	// 掲示板: その掲示板に立っているスレッド。カテゴリーと同じ理由でサインアウト
	// 状態でも読め、同じように登録する。
	r.With(authMiddleware.SetUser).Get("/b/{slug}", boardHandler.Show)

	// スレッドを立てる: その掲示板の新しいスレッドを書くフォーム。書き込めるのは
	// サインイン済みの訪問者だけであるためRequireAuthの背後に置き、匿名の訪問者は
	// このアドレスを載せてサインインへ送られ、フォームへ戻ってくる。掲示板はフィールドでは
	// なくアドレスの一部であるため、フォームはそれが開かれた掲示板へ送信する。
	r.With(authMiddleware.RequireAuth).Get("/b/{slug}/threads/new", threadHandler.New)

	// 掲示板のスレッド: 上のフォームが新しいスレッドを送信する先のコレクション。
	// フォームと同じ理由でRequireAuthの背後に置く。ここへは書き込むだけである。掲示板の
	// スレッドは /b/{slug} で読み、各スレッドは自身のidのアドレスで読むため、スレッドを
	// 移してもそこへのリンクは保たれる。
	r.With(authMiddleware.RequireAuth).Post("/b/{slug}/threads", threadHandler.Create)

	// スレッド: そのスレッドに書かれた投稿。掲示板と同じ理由でサインアウト状態でも
	// 読め、同じように登録する。
	r.With(authMiddleware.SetUser).Get("/t/{id}", threadHandler.Show)

	// スレッドの投稿: スレッドの末尾の返信フォームが送信する先のコレクション。
	// 書き込めるのはサインイン済みの訪問者だけであるためRequireAuthの背後に置く。
	// ここへは書き込むだけである。スレッドの投稿は /t/{id} で読み、そこでは各投稿が
	// レス番号で名指される (ADR 0009)。
	r.With(authMiddleware.RequireAuth).Post("/t/{id}/posts", postHandler.Create)

	// スレッドのロック: スレッドがそこから閉じられる確認ページ、閉じること、そして
	// ロックを外すこと。3つともRequireAuthの背後に置く。ロールを持てるのはサインイン済みの
	// 訪問者だけであり、そのサインインした人がこのスレッドに働きかけてよいかどうかを決めるのは
	// UseCaseである。掛けることと外すことはロック自身のアドレスを共有し、外すほうはスレッドの
	// ページから_method=DELETEのオーバーライドで到達する。
	r.With(authMiddleware.RequireAuth).Get("/t/{id}/lock/new", threadLockHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/t/{id}/lock", threadLockHandler.Create)
	r.With(authMiddleware.RequireAuth).Delete("/t/{id}/lock", threadLockHandler.Delete)

	// スレッドの非公開: スレッドがそこからコミュニティの視界の外へ移される確認ページと、
	// 印そのもの。どちらもRequireAuthの背後に置くのはロックのルートと同じ理由である。この組は
	// ロックの2つの書き込みルートと同じ形をしている。印が自身のアドレスを持ち、それを確認する
	// ページがそのアドレスの下に /newとして下がる。
	r.With(authMiddleware.RequireAuth).Get("/t/{id}/unpublication/new", threadUnpublicationHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/t/{id}/unpublication", threadUnpublicationHandler.Create)

	// 投稿1件の非公開: 投稿1件がそこから視界の外へ移される確認ページと、印そのもの。
	// 投稿はスレッドの投稿の下でレス番号によって名指される。それが、投稿が参照されるあらゆる
	// 場所での名指し方であるためである (ADR 0009)。アドレスから投稿を読むルートはこれだけで
	// ある。スレッドの投稿は /t/{id} で読まれるためである。
	r.With(authMiddleware.RequireAuth).Get("/t/{id}/posts/{number}/unpublication/new", postUnpublicationHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/t/{id}/posts/{number}/unpublication", postUnpublicationHandler.Create)

	// サインアップ: フォームを表示し、確認コード発行のためemailを受け付ける。
	r.Get("/sign_up", signUpHandler.New)
	r.Post("/sign_up", signUpHandler.Create)

	// メール確認: コード入力フォームを表示し、サインアップ時にメールした
	// コードを検証する。
	r.Get("/email_confirmation/new", emailConfirmationHandler.New)
	r.Post("/email_confirmation", emailConfirmationHandler.Create)

	// アカウント作成: パスワード設定フォームを表示してアカウントを作成し、
	// ユーザーをサインインさせる。
	r.Get("/account/new", accountHandler.New)
	r.Post("/account", accountHandler.Create)

	// サインイン: フォームを表示し、emailとパスワードを認証して、成功時に
	// セッションを発行する。
	r.Get("/sign_in", signInHandler.New)
	r.Post("/sign_in", signInHandler.Create)

	// サインインの2段階認証チャレンジ: TOTPコード入力フォームを表示し、コードを検証して
	// 2FA有効なアカウントのサインインを完了させ、成功時にセッションを発行する。これらは
	// サインインの途中で通る公開ルートで、保留中ユーザーはセッションではなくパスワードのステップが
	// 設定した短命の2段階認証Cookieから解決する。
	r.Get("/sign_in/two_factor/new", signInTwoFactorHandler.New)
	r.Post("/sign_in/two_factor", signInTwoFactorHandler.Create)

	// サインインの2段階認証リカバリーコードチャレンジ: 認証アプリを使えないときに
	// リカバリーコード入力フォームを表示し、コードを検証してサインインを完了させ、成功時に
	// 1回使い切りのコードを消費してセッションを発行する。TOTPチャレンジと同様、これらは
	// サインインの途中で通る公開ルートで、保留中ユーザーはセッションではなく短命の2段階認証
	// Cookieから解決する。
	r.Get("/sign_in/two_factor/recovery/new", signInTwoFactorRecoveryHandler.New)
	r.Post("/sign_in/two_factor/recovery", signInTwoFactorRecoveryHandler.Create)

	// サインアウト: 現在のセッションを削除しセッションCookieを消去する。
	r.Delete("/user_session", userSessionHandler.Delete)

	// パスワードリセット申請: フォームを表示し、リセットリンク発行のためemailを
	// 受け付ける。リンクはアカウントが存在すればそのアカウントへメールされる。
	r.Get("/password_reset/new", passwordResetHandler.New)
	r.Post("/password_reset", passwordResetHandler.Create)

	// パスワードリセット更新: メールのリンクから新パスワードフォームを表示し、新しい
	// パスワードを設定してリセットトークンを消費する。フォームは _methodオーバーライドで
	// PATCHを動かす。
	r.Get("/password/edit", passwordHandler.Edit)
	r.Patch("/password", passwordHandler.Update)

	// 設定ハブ: 各設定画面 (今はメールアドレス変更) へリンクする着地ページ。
	// RequireAuthの背後に置く。
	r.With(authMiddleware.RequireAuth).Get("/settings", settingsHandler.Show)

	// 設定 — メールアドレス変更: 変更フォーム (現在のアドレス付き) を表示し、新しい
	// emailと現在のパスワードを受け付けて確認コードを発行する。どちらもRequireAuthの
	// 背後に置き、フォームは _methodオーバーライドでPATCHを動かす。
	r.With(authMiddleware.RequireAuth).Get("/settings/email/edit", settingsEmailHandler.Edit)
	r.With(authMiddleware.RequireAuth).Patch("/settings/email", settingsEmailHandler.Update)

	// 設定 — メールアドレス変更の確認: コード入力フォームを表示し、新しいアドレスに
	// メールしたコードを検証する。成功時に変更を適用する。どちらもRequireAuthの背後に置き、
	// 保留中の確認は受け渡しCookieではなくサインイン済みユーザーから解決する。
	r.With(authMiddleware.RequireAuth).Get("/settings/email/confirmation/new", settingsEmailConfirmationHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/settings/email/confirmation", settingsEmailConfirmationHandler.Create)

	// 設定 — 2段階認証: 2FAが無効なら登録フォーム (QRコードと手動入力キー) を、有効なら
	// 無効化の確認フォームを表示し、ユーザーがTOTPコードを確認した後に2FAを有効化する。
	// 有効化は設定をアクティブにし、1回使い切りのリカバリーコードを表示する。無効化は再認証
	// (現在のパスワードか現在のTOTPコード) の後に設定を削除する。すべてRequireAuthの背後に
	// 置き、設定 (GET) は登録 / 無効化フォームを、有効化 (POST) は素のPOST、無効化 (DELETE) は
	// 無効化フォームから _methodオーバーライドで到達する。
	r.With(authMiddleware.RequireAuth).Get("/settings/two_factor_auth/new", settingsTwoFactorAuthHandler.New)
	r.With(authMiddleware.RequireAuth).Post("/settings/two_factor_auth", settingsTwoFactorAuthHandler.Create)
	r.With(authMiddleware.RequireAuth).Delete("/settings/two_factor_auth", settingsTwoFactorAuthHandler.Delete)

	// 設定 — 退会: 確認フォーム (現在のパスワードフィールド付き) を表示し、退会を実行する。
	// 退会の実行はアカウントを論理削除・匿名化し、その全セッションを削除する。どちらも
	// RequireAuthの背後に置き、フォームは _methodオーバーライドでDELETEを動かす。設定ハブ
	// からのリンクはまだ無い (後続タスクで追加) ため、このページはURL直打ちでのみ到達する。
	r.With(authMiddleware.RequireAuth).Get("/settings/withdrawal/new", settingsWithdrawalHandler.New)
	r.With(authMiddleware.RequireAuth).Delete("/settings/withdrawal", settingsWithdrawalHandler.Delete)

	// 管理: ハブ・利用者一覧・一覧が送信するロールの書き込み。ルートとそれぞれの
	// 応答はregisterAdminRoutesに記す。
	registerAdminRoutes(r, authMiddleware, adminHandler, adminUserHandler, adminUserRoleHandler)

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

	// グレースフルシャットダウン。SIGINT / SIGTERMを受けたら新規接続の
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

	// 終了する前に処理中リクエストのドレイン完了を待つ。これが無いと、
	// goroutine内のsrv.Shutdownがまだ接続をドレインしている最中でも、
	// ListenAndServeがErrServerClosedを返した時点でmainが返ってしまう。
	<-shutdownDone
	slog.Info("the server has stopped")
}
