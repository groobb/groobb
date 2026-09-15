package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newEmailConfirmationRepoはテストが所有するデータベース上に
// EmailConfirmationRepositoryを作る。リポジトリだけが必要なテストがデータベース自体を
// 抱えずに済むようにするためである。
func newEmailConfirmationRepo(t *testing.T) (*repository.EmailConfirmationRepository, context.Context) {
	t.Helper()
	db := testutil.SetupDB(t)
	return repository.NewEmailConfirmationRepository(db), context.Background()
}

func TestEmailConfirmationRepository_Create(t *testing.T) {
	t.Parallel()

	repo, ctx := newEmailConfirmationRepo(t)

	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: "create@example.com",
		Event: model.EmailConfirmationEventSignUp,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if confirmation.ID == 0 {
		t.Error("Create() confirmation.IDはDB採番で空でないはず")
	}
	if confirmation.UserID != nil {
		t.Errorf("confirmation.UserID = %v、期待値 = nil (サインアップ確認はユーザー未紐付け)", confirmation.UserID)
	}
	if confirmation.Email != "create@example.com" {
		t.Errorf("confirmation.Email = %q、期待値 = %q", confirmation.Email, "create@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventSignUp {
		t.Errorf("confirmation.Event = %q、期待値 = %q", confirmation.Event, model.EmailConfirmationEventSignUp)
	}
	if confirmation.Code != "123456" {
		t.Errorf("confirmation.Code = %q、期待値 = %q", confirmation.Code, "123456")
	}
	if confirmation.StartedAt.IsZero() {
		t.Error("confirmation.StartedAtはDB既定値で設定されるはず")
	}
	if confirmation.SucceededAt != nil {
		t.Errorf("confirmation.SucceededAt = %v、期待値 = nil (作成直後は未確認)", confirmation.SucceededAt)
	}
	if confirmation.CreatedAt.IsZero() {
		t.Error("confirmation.CreatedAtはDB既定値で設定されるはず")
	}
	if confirmation.UpdatedAt.IsZero() {
		t.Error("confirmation.UpdatedAtはDB既定値で設定されるはず")
	}
}

// TestEmailConfirmationRepository_CreatePreservesEmailCaseはNOCASE照合のemail
// 列が与えたとおりにアドレスを保存することを確認する (確認はユーザーが入力した正確な
// アドレスをキーとする)。照合は他の場面では大文字小文字を無視する。
func TestEmailConfirmationRepository_CreatePreservesEmailCase(t *testing.T) {
	t.Parallel()

	repo, ctx := newEmailConfirmationRepo(t)

	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: "Mixed.Case@Example.com",
		Event: model.EmailConfirmationEventSignUp,
		Code:  "654321",
	})
	if err != nil {
		t.Fatalf("Create()のエラー = %v", err)
	}

	if confirmation.Email != "Mixed.Case@Example.com" {
		t.Errorf("confirmation.Email = %q、期待値 = %q", confirmation.Email, "Mixed.Case@Example.com")
	}
}

// TestEmailConfirmationRepository_FindActiveByIDは "active" フィルタを網羅する。
// 発行直後の確認は返り、未知のid・確認済み・15分のウィンドウ外で発行された確認は
// いずれも (nil, nil) になる。
func TestEmailConfirmationRepository_FindActiveByID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	t.Run("発行直後の確認はactiveとして返る", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, db).WithCode("123456").Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got == nil {
			t.Fatal("発行直後の確認は返るはず (nilが返った)")
		}
		if got.ID != id {
			t.Errorf("got.ID = %v、期待値 = %v", got.ID, id)
		}
		if got.Code != "123456" {
			t.Errorf("got.Code = %q、期待値 = %q", got.Code, "123456")
		}
	})

	t.Run("未知のidはnil", func(t *testing.T) {
		got, err := repo.FindActiveByID(ctx, model.EmailConfirmationID(testutil.UnusedID))
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("未知のidはnilを返すはず: %+v", got)
		}
	})

	t.Run("確認済み (succeeded_atあり) はactiveでない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, db).Build()
		if err := repo.Succeed(ctx, id); err != nil {
			t.Fatalf("Succeed()のエラー = %v", err)
		}

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("確認済みはactiveでないはず: %+v", got)
		}
	})

	t.Run("有効期限切れ (started_atが15分より前) はactiveでない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, db).
			WithStartedAt(time.Now().Add(-16 * time.Minute)).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("期限切れはactiveでないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限に達した確認はactiveでない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, db).
			WithFailedAttemptsCount(5).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("試行回数超過はactiveでないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限未満の確認はactiveとして返り回数も読める", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, db).
			WithFailedAttemptsCount(4).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID()のエラー = %v", err)
		}
		if got == nil {
			t.Fatal("上限未満の確認はactiveとして返るはず (nilが返った)")
		}
		if got.FailedAttemptsCount != 4 {
			t.Errorf("got.FailedAttemptsCount = %d、期待値 = 4", got.FailedAttemptsCount)
		}
	})
}

// TestEmailConfirmationRepository_IncrementFailedAttemptsは、各呼び出しが
// failed_attempts_countを1ずつ増やすことを検証する。回数はFindActiveByID (上限未満の
// 間は行を返す) 経由で読み戻す。
func TestEmailConfirmationRepository_IncrementFailedAttempts(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	id := testutil.NewEmailConfirmationBuilder(t, db).Build()

	if err := repo.IncrementFailedAttempts(ctx, id); err != nil {
		t.Fatalf("IncrementFailedAttempts()のエラー = %v", err)
	}
	got, err := repo.FindActiveByID(ctx, id)
	if err != nil {
		t.Fatalf("FindActiveByID()のエラー = %v", err)
	}
	if got == nil {
		t.Fatal("1回のインクリメント後はまだactiveのはず")
	}
	if got.FailedAttemptsCount != 1 {
		t.Errorf("1回のインクリメント後のFailedAttemptsCount = %d、期待値 = 1", got.FailedAttemptsCount)
	}

	if err := repo.IncrementFailedAttempts(ctx, id); err != nil {
		t.Fatalf("IncrementFailedAttempts()のエラー = %v", err)
	}
	got, err = repo.FindActiveByID(ctx, id)
	if err != nil {
		t.Fatalf("FindActiveByID()のエラー = %v", err)
	}
	if got == nil {
		t.Fatal("2回のインクリメント後はまだactiveのはず")
	}
	if got.FailedAttemptsCount != 2 {
		t.Errorf("2回のインクリメント後のFailedAttemptsCount = %d、期待値 = 2", got.FailedAttemptsCount)
	}
}

// TestEmailConfirmationRepository_SucceedはSucceedが行のsucceeded_atを打刻する
// ことを検証する。直接読み戻してNULLでなくなったことを確認する。
func TestEmailConfirmationRepository_Succeed(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	id := testutil.NewEmailConfirmationBuilder(t, db).Build()

	if err := repo.Succeed(ctx, id); err != nil {
		t.Fatalf("Succeed()のエラー = %v", err)
	}

	var succeededAt *time.Time
	err := db.Writer.QueryRowContext(ctx, `SELECT succeeded_at FROM email_confirmations WHERE id = ?`, int64(id)).Scan(&succeededAt)
	if err != nil {
		t.Fatalf("succeeded_atの読み戻しに失敗: %v", err)
	}
	if succeededAt == nil {
		t.Error("Succeed() はsucceeded_atを打刻するはず (NULLのまま)")
	}
}

// TestEmailConfirmationRepository_CreateEmailChangeは、メール変更の確認が申請した
// ユーザーに紐付いて挿入され、eventがemail_changeに固定され、新しいアドレスがemailに
// 保存されることを検証する。
func TestEmailConfirmationRepository_CreateEmailChange(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, db).Build()

	confirmation, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{
		UserID: userID,
		Email:  "new-address@example.com",
		Code:   "123456",
	})
	if err != nil {
		t.Fatalf("CreateEmailChange()のエラー = %v", err)
	}

	if confirmation.ID == 0 {
		t.Error("CreateEmailChange() confirmation.IDはDB採番で空でないはず")
	}
	if confirmation.UserID == nil {
		t.Fatal("confirmation.UserIDは設定されるはず (nilが返った)")
	}
	if *confirmation.UserID != userID {
		t.Errorf("*confirmation.UserID = %v、期待値 = %v", *confirmation.UserID, userID)
	}
	if confirmation.Email != "new-address@example.com" {
		t.Errorf("confirmation.Email = %q、期待値 = %q", confirmation.Email, "new-address@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventEmailChange {
		t.Errorf("confirmation.Event = %q、期待値 = %q", confirmation.Event, model.EmailConfirmationEventEmailChange)
	}
	if confirmation.Code != "123456" {
		t.Errorf("confirmation.Code = %q、期待値 = %q", confirmation.Code, "123456")
	}
	if confirmation.SucceededAt != nil {
		t.Errorf("confirmation.SucceededAt = %v、期待値 = nil (作成直後は未確認)", confirmation.SucceededAt)
	}
	if confirmation.StartedAt.IsZero() {
		t.Error("confirmation.StartedAtはDB既定値で設定されるはず")
	}
}

// TestEmailConfirmationRepository_FindActiveEmailChangeByUserIDはメール変更確認の
// ユーザー単位 "active" フィルタを網羅する。発行直後のものは返り、保留中の無いユーザー・
// サインアップ確認 (event違い)・確認済み・期限切れ・試行超過はいずれも (nil, nil) になる。
func TestEmailConfirmationRepository_FindActiveEmailChangeByUserID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	t.Run("発行直後のメール変更確認はactiveとして返る", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		id := testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithCode("123456").
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got == nil {
			t.Fatal("発行直後の確認は返るはず (nilが返った)")
		}
		if got.ID != id {
			t.Errorf("got.ID = %v、期待値 = %v", got.ID, id)
		}
		if got.UserID == nil || *got.UserID != userID {
			t.Errorf("got.UserID = %v、期待値 = %v", got.UserID, userID)
		}
		if got.Code != "123456" {
			t.Errorf("got.Code = %q、期待値 = %q", got.Code, "123456")
		}
	})

	t.Run("保留中の確認が無いユーザーはnil", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("保留中の無いユーザーはnilを返すはず: %+v", got)
		}
	})

	t.Run("サインアップ確認 (event違い) は返さない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		// user_idを紐付けてもeventがsign_upの確認はemail_changeの
		// ルックアップにヒットしない。
		testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventSignUp).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("eventがsign_upの確認はemail_changeとして返らないはず: %+v", got)
		}
	})

	t.Run("確認済み (succeeded_atあり) はactiveでない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		id := testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()
		if err := repo.Succeed(ctx, id); err != nil {
			t.Fatalf("Succeed()のエラー = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("確認済みはactiveでないはず: %+v", got)
		}
	})

	t.Run("有効期限切れ (started_atが15分より前) はactiveでない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithStartedAt(time.Now().Add(-16 * time.Minute)).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("期限切れはactiveでないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限に達した確認はactiveでない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithFailedAttemptsCount(5).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("試行回数超過はactiveでないはず: %+v", got)
		}
	})
}

// TestEmailConfirmationRepository_DeleteUnusedEmailChangesByUserIDは、削除が
// ユーザーごとに保留中を高々1件に保つことを検証する。ユーザーの未確認のメール変更確認を
// 削除し、確認済みは記録として残し、他ユーザーの確認には触れず、保留中が無ければno-opに
// なる。
func TestEmailConfirmationRepository_DeleteUnusedEmailChangesByUserID(t *testing.T) {
	t.Parallel()

	db := testutil.SetupDB(t)
	repo := repository.NewEmailConfirmationRepository(db)
	ctx := context.Background()

	t.Run("未確認のメール変更確認を削除する", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID()のエラー = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got != nil {
			t.Errorf("削除後は保留中の確認が無いはず: %+v", got)
		}
	})

	t.Run("確認済みのものは残す", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		succeededID := testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()
		if err := repo.Succeed(ctx, succeededID); err != nil {
			t.Fatalf("Succeed()のエラー = %v", err)
		}

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID()のエラー = %v", err)
		}

		// 確認済みの行が残っていることを直接確認する。
		var count int
		if err := db.Writer.QueryRowContext(ctx, `SELECT COUNT(*) FROM email_confirmations WHERE id = ?`, int64(succeededID)).Scan(&count); err != nil {
			t.Fatalf("確認済み行の件数取得に失敗: %v", err)
		}
		if count != 1 {
			t.Errorf("確認済みの行は削除されないはず: count = %d、期待値 = 1", count)
		}
	})

	t.Run("他ユーザーの確認は削除しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()
		otherUserID := testutil.NewUserBuilder(t, db).Build()
		testutil.NewEmailConfirmationBuilder(t, db).
			WithUserID(otherUserID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID()のエラー = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, otherUserID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID()のエラー = %v", err)
		}
		if got == nil {
			t.Error("他ユーザーの保留中確認は残るはず (nilが返った)")
		}
	})

	t.Run("保留中が無くてもエラーにならない (no-op)", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, db).Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("保留中が無いときのDeleteUnusedEmailChangesByUserID()のエラー = %v", err)
		}
	})
}
