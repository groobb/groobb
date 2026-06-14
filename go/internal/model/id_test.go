package model_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/model"
)

// TestUserID_String verifies that a UserID stringifies to the canonical UUID
// form of the uuid.UUID it wraps.
//
// [Ja] TestUserID_String は UserID がラップする uuid.UUID の正準 UUID 形式で
// 文字列化されることを検証します。
func TestUserID_String(t *testing.T) {
	t.Parallel()

	u := uuid.New()
	id := model.UserID(u)

	if got, want := id.String(), u.String(); got != want {
		t.Errorf("UserID.String() = %q, want %q", got, want)
	}
}

// TestUserIDSliceConversion verifies that converting UserIDs to uuid.UUIDs and
// back yields the original values in order.
//
// [Ja] TestUserIDSliceConversion は UserID スライスを uuid.UUID スライスに変換して
// 戻すと、元の値が順序どおりに復元されることを検証します。
func TestUserIDSliceConversion(t *testing.T) {
	t.Parallel()

	ids := []model.UserID{
		model.UserID(uuid.New()),
		model.UserID(uuid.New()),
	}

	got := model.UUIDsToUserIDs(model.UserIDsToUUIDs(ids))

	if len(got) != len(ids) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(ids))
	}
	for i := range ids {
		if got[i] != ids[i] {
			t.Errorf("got[%d] = %v, want %v", i, got[i], ids[i])
		}
	}
}
