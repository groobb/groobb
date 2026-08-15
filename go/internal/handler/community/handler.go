// Package community provides the handlers for a community: showing the creation
// form (GET /communities/new), creating the community, which also makes its
// creator the first member and its administrator (POST /communities), and the
// community's own page (GET /c/{identifier}). Every route is registered behind
// RequireAuth, so a community always has a signed-in creator to found it and its
// page is visible to signed-in visitors only.
//
// [Ja] community パッケージはコミュニティのハンドラーを提供します。作成フォームの表示
// (GET /communities/new)、コミュニティの作成 (POST /communities。作成者を最初のメンバー
// かつ管理者にもします)、そしてコミュニティ自身の画面 (GET /c/{identifier}) です。いずれの
// ルートも RequireAuth の背後に登録されるため、コミュニティには常にそれを作成するサインイン
// 済みの作成者がおり、その画面はサインイン済みの訪問者にのみ見えます。
package community

import (
	"github.com/groobb/groobb/go/internal/config"
	"github.com/groobb/groobb/go/internal/session"
	"github.com/groobb/groobb/go/internal/usecase"
)

// Handler is the HTTP handler for community creation and display.
//
// [Ja] Handler はコミュニティの作成と表示を担う HTTP ハンドラーです。
type Handler struct {
	cfg               *config.Config
	flashMgr          *session.FlashManager
	createCommunityUC *usecase.CreateCommunityUsecase
	getCommunityUC    *usecase.GetCommunityUsecase
}

// NewHandler creates a community Handler.
//
// [Ja] NewHandler はコミュニティ Handler を生成します。
func NewHandler(
	cfg *config.Config,
	flashMgr *session.FlashManager,
	createCommunityUC *usecase.CreateCommunityUsecase,
	getCommunityUC *usecase.GetCommunityUsecase,
) *Handler {
	return &Handler{
		cfg:               cfg,
		flashMgr:          flashMgr,
		createCommunityUC: createCommunityUC,
		getCommunityUC:    getCommunityUC,
	}
}
