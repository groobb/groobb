package policy_test

import (
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/policy"
)

// TestCommunityPolicy verifies what each set of scopes is admitted to. The cases
// cover holding nothing, holding one scope of the vocabulary, holding
// community:admin, and holding a name the vocabulary does not define, which is
// what a database migrated ahead of this binary hands over.
//
// [Ja] TestCommunityPolicy は、それぞれのスコープの集合が何を許されるかを検証します。
// 何も持たない場合、語彙のスコープを 1 つ持つ場合、community:admin を持つ場合、そして
// 語彙が定義しない名前を持つ場合を扱います。最後のものは、このバイナリより先に
// マイグレートされたデータベースが渡してくるものです。
func TestCommunityPolicy(t *testing.T) {
	t.Parallel()

	// unknownScope stands for a scope a later build defines, read back by this
	// one from roles.scopes.
	//
	// [Ja] unknownScope は、後のビルドが定義するスコープを表します。このビルドはそれを
	// roles.scopes から読み戻します。
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
			name:   "スコープを 1 つも持たない",
			scopes: nil,
		},
		{
			name:            "user:read だけを持つ",
			scopes:          []model.Scope{model.ScopeUserRead},
			wantAccessAdmin: true,
			wantListUsers:   true,
		},
		{
			name:               "user_role:write だけを持つ",
			scopes:             []model.Scope{model.ScopeUserRoleWrite},
			wantAccessAdmin:    true,
			wantGrantUserRole:  true,
			wantRevokeUserRole: true,
		},
		{
			name:               "thread_lock:write だけを持つ",
			scopes:             []model.Scope{model.ScopeThreadLockWrite},
			wantLockThread:     true,
			wantUnlockThread:   true,
			wantModerateThread: true,
		},
		{
			name:                "thread_unpublication:write だけを持つ",
			scopes:              []model.Scope{model.ScopeThreadUnpublicationWrite},
			wantUnpublishThread: true,
			wantModerateThread:  true,
		},
		{
			name:               "post_unpublication:write だけを持つ",
			scopes:             []model.Scope{model.ScopePostUnpublicationWrite},
			wantUnpublishPost:  true,
			wantModerateThread: true,
		},
		{
			name:              "user_suspension:write だけを持つ",
			scopes:            []model.Scope{model.ScopeUserSuspensionWrite},
			wantSuspendUser:   true,
			wantUnsuspendUser: true,
		},
		{
			name:                   "moderation_log:read だけを持つ",
			scopes:                 []model.Scope{model.ScopeModerationLogRead},
			wantAccessAdmin:        true,
			wantListModerationLogs: true,
		},
		{
			name:               "語彙の 2 つのスコープを別々のロールから合わせて持つ",
			scopes:             []model.Scope{model.ScopeUserRead, model.ScopeUserRoleWrite},
			wantAccessAdmin:    true,
			wantListUsers:      true,
			wantGrantUserRole:  true,
			wantRevokeUserRole: true,
		},
		{
			name:                "モデレーションの 2 つのスコープを別々のロールから合わせて持つ",
			scopes:              []model.Scope{model.ScopeThreadLockWrite, model.ScopeThreadUnpublicationWrite},
			wantLockThread:      true,
			wantUnlockThread:    true,
			wantUnpublishThread: true,
			wantModerateThread:  true,
		},
		{
			name:                   "community:admin を持つ",
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
			name:            "語彙に無いスコープと user:read を持つ",
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
				t.Errorf("CanAccessAdmin() = %v, want %v", got, tt.wantAccessAdmin)
			}
			if got := p.CanListUsers(); got != tt.wantListUsers {
				t.Errorf("CanListUsers() = %v, want %v", got, tt.wantListUsers)
			}
			if got := p.CanGrantUserRole(); got != tt.wantGrantUserRole {
				t.Errorf("CanGrantUserRole() = %v, want %v", got, tt.wantGrantUserRole)
			}
			if got := p.CanRevokeUserRole(); got != tt.wantRevokeUserRole {
				t.Errorf("CanRevokeUserRole() = %v, want %v", got, tt.wantRevokeUserRole)
			}
			if got := p.CanLockThread(); got != tt.wantLockThread {
				t.Errorf("CanLockThread() = %v, want %v", got, tt.wantLockThread)
			}
			if got := p.CanUnlockThread(); got != tt.wantUnlockThread {
				t.Errorf("CanUnlockThread() = %v, want %v", got, tt.wantUnlockThread)
			}
			if got := p.CanUnpublishThread(); got != tt.wantUnpublishThread {
				t.Errorf("CanUnpublishThread() = %v, want %v", got, tt.wantUnpublishThread)
			}
			if got := p.CanUnpublishPost(); got != tt.wantUnpublishPost {
				t.Errorf("CanUnpublishPost() = %v, want %v", got, tt.wantUnpublishPost)
			}
			if got := p.CanModerateThread(); got != tt.wantModerateThread {
				t.Errorf("CanModerateThread() = %v, want %v", got, tt.wantModerateThread)
			}
			if got := p.CanSuspendUser(); got != tt.wantSuspendUser {
				t.Errorf("CanSuspendUser() = %v, want %v", got, tt.wantSuspendUser)
			}
			if got := p.CanUnsuspendUser(); got != tt.wantUnsuspendUser {
				t.Errorf("CanUnsuspendUser() = %v, want %v", got, tt.wantUnsuspendUser)
			}
			if got := p.CanListModerationLogs(); got != tt.wantListModerationLogs {
				t.Errorf("CanListModerationLogs() = %v, want %v", got, tt.wantListModerationLogs)
			}
		})
	}
}
