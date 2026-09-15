package policy_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/policy"
)

// TestCommunityPolicyは、それぞれのスコープの集合が何を許されるかを検証します。
// 何も持たない場合、語彙のスコープを1つ持つ場合、community:adminを持つ場合、そして
// 語彙が定義しない名前を持つ場合を扱います。最後のものは、このバイナリより先に
// マイグレートされたデータベースが渡してくるものです。
func TestCommunityPolicy(t *testing.T) {
	t.Parallel()

	// unknownScopeは、後のビルドが定義するスコープを表します。このビルドはそれを
	// roles.scopesから読み戻します。
	const unknownScope = model.Scope("report:read")

	tests := []struct {
		name                   string
		scopes                 []model.Scope
		wantAccessAdmin        bool
		wantListUsers          bool
		wantGrantUserRole      bool
		wantRevokeUserRole     bool
		wantLockThread         bool
		wantUnlockThread       bool
		wantUnpublishThread    bool
		wantUnpublishPost      bool
		wantModerateThread     bool
		wantSuspendUser        bool
		wantUnsuspendUser      bool
		wantListModerationLogs bool
	}{
		{
			name:   "スコープを1つも持たない",
			scopes: nil,
		},
		{
			name:            "user:readだけを持つ",
			scopes:          []model.Scope{model.ScopeUserRead},
			wantAccessAdmin: true,
			wantListUsers:   true,
		},
		{
			name:               "user_role:writeだけを持つ",
			scopes:             []model.Scope{model.ScopeUserRoleWrite},
			wantAccessAdmin:    true,
			wantGrantUserRole:  true,
			wantRevokeUserRole: true,
		},
		{
			name:               "thread_lock:writeだけを持つ",
			scopes:             []model.Scope{model.ScopeThreadLockWrite},
			wantLockThread:     true,
			wantUnlockThread:   true,
			wantModerateThread: true,
		},
		{
			name:                "thread_unpublication:writeだけを持つ",
			scopes:              []model.Scope{model.ScopeThreadUnpublicationWrite},
			wantUnpublishThread: true,
			wantModerateThread:  true,
		},
		{
			name:               "post_unpublication:writeだけを持つ",
			scopes:             []model.Scope{model.ScopePostUnpublicationWrite},
			wantUnpublishPost:  true,
			wantModerateThread: true,
		},
		{
			name:              "user_suspension:writeだけを持つ",
			scopes:            []model.Scope{model.ScopeUserSuspensionWrite},
			wantSuspendUser:   true,
			wantUnsuspendUser: true,
		},
		{
			name:                   "moderation_log:readだけを持つ",
			scopes:                 []model.Scope{model.ScopeModerationLogRead},
			wantAccessAdmin:        true,
			wantListModerationLogs: true,
		},
		{
			name:               "語彙の2つのスコープを別々のロールから合わせて持つ",
			scopes:             []model.Scope{model.ScopeUserRead, model.ScopeUserRoleWrite},
			wantAccessAdmin:    true,
			wantListUsers:      true,
			wantGrantUserRole:  true,
			wantRevokeUserRole: true,
		},
		{
			name:                "モデレーションの2つのスコープを別々のロールから合わせて持つ",
			scopes:              []model.Scope{model.ScopeThreadLockWrite, model.ScopeThreadUnpublicationWrite},
			wantLockThread:      true,
			wantUnlockThread:    true,
			wantUnpublishThread: true,
			wantModerateThread:  true,
		},
		{
			name:                   "community:adminを持つ",
			scopes:                 []model.Scope{model.ScopeCommunityAdmin},
			wantAccessAdmin:        true,
			wantListUsers:          true,
			wantGrantUserRole:      true,
			wantRevokeUserRole:     true,
			wantLockThread:         true,
			wantUnlockThread:       true,
			wantUnpublishThread:    true,
			wantUnpublishPost:      true,
			wantModerateThread:     true,
			wantSuspendUser:        true,
			wantUnsuspendUser:      true,
			wantListModerationLogs: true,
		},
		{
			name:   "語彙に無いスコープだけを持つ",
			scopes: []model.Scope{unknownScope},
		},
		{
			name:            "語彙に無いスコープとuser:readを持つ",
			scopes:          []model.Scope{unknownScope, model.ScopeUserRead},
			wantAccessAdmin: true,
			wantListUsers:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := policy.NewCommunityPolicy(tt.scopes)

			if got := p.CanAccessAdmin(); got != tt.wantAccessAdmin {
				t.Errorf("CanAccessAdmin() = %v、期待値 = %v", got, tt.wantAccessAdmin)
			}
			if got := p.CanListUsers(); got != tt.wantListUsers {
				t.Errorf("CanListUsers() = %v、期待値 = %v", got, tt.wantListUsers)
			}
			if got := p.CanGrantUserRole(); got != tt.wantGrantUserRole {
				t.Errorf("CanGrantUserRole() = %v、期待値 = %v", got, tt.wantGrantUserRole)
			}
			if got := p.CanRevokeUserRole(); got != tt.wantRevokeUserRole {
				t.Errorf("CanRevokeUserRole() = %v、期待値 = %v", got, tt.wantRevokeUserRole)
			}
			if got := p.CanLockThread(); got != tt.wantLockThread {
				t.Errorf("CanLockThread() = %v、期待値 = %v", got, tt.wantLockThread)
			}
			if got := p.CanUnlockThread(); got != tt.wantUnlockThread {
				t.Errorf("CanUnlockThread() = %v、期待値 = %v", got, tt.wantUnlockThread)
			}
			if got := p.CanUnpublishThread(); got != tt.wantUnpublishThread {
				t.Errorf("CanUnpublishThread() = %v、期待値 = %v", got, tt.wantUnpublishThread)
			}
			if got := p.CanUnpublishPost(); got != tt.wantUnpublishPost {
				t.Errorf("CanUnpublishPost() = %v、期待値 = %v", got, tt.wantUnpublishPost)
			}
			if got := p.CanModerateThread(); got != tt.wantModerateThread {
				t.Errorf("CanModerateThread() = %v、期待値 = %v", got, tt.wantModerateThread)
			}
			if got := p.CanSuspendUser(); got != tt.wantSuspendUser {
				t.Errorf("CanSuspendUser() = %v、期待値 = %v", got, tt.wantSuspendUser)
			}
			if got := p.CanUnsuspendUser(); got != tt.wantUnsuspendUser {
				t.Errorf("CanUnsuspendUser() = %v、期待値 = %v", got, tt.wantUnsuspendUser)
			}
			if got := p.CanListModerationLogs(); got != tt.wantListModerationLogs {
				t.Errorf("CanListModerationLogs() = %v、期待値 = %v", got, tt.wantListModerationLogs)
			}
		})
	}
}
