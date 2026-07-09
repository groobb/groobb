package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/query"
	"github.com/groobb/groobb/go/internal/repository"
	"github.com/groobb/groobb/go/internal/testutil"
)

// newEmailConfirmationRepo builds an EmailConfirmationRepository bound to the
// test transaction, exercising WithTx so writes roll back when the test
// finishes.
//
// [Ja] newEmailConfirmationRepo はテスト用トランザクションに束ねた
// EmailConfirmationRepository を作る。WithTx を通すことで、テスト終了時に書き込みが
// ロールバックされる。
func newEmailConfirmationRepo(t *testing.T) (*repository.EmailConfirmationRepository, context.Context) {
	t.Helper()
	db, tx := testutil.SetupTx(t)
	return repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx), context.Background()
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
		t.Fatalf("Create() error = %v", err)
	}

	if confirmation.ID == (model.EmailConfirmationID{}) {
		t.Error("Create() confirmation.ID は DB 採番で空でないはず")
	}
	if confirmation.UserID != nil {
		t.Errorf("confirmation.UserID = %v, want nil (サインアップ確認はユーザー未紐付け)", confirmation.UserID)
	}
	if confirmation.Email != "create@example.com" {
		t.Errorf("confirmation.Email = %q, want %q", confirmation.Email, "create@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventSignUp {
		t.Errorf("confirmation.Event = %q, want %q", confirmation.Event, model.EmailConfirmationEventSignUp)
	}
	if confirmation.Code != "123456" {
		t.Errorf("confirmation.Code = %q, want %q", confirmation.Code, "123456")
	}
	if confirmation.StartedAt.IsZero() {
		t.Error("confirmation.StartedAt は DB 既定値で設定されるはず")
	}
	if confirmation.SucceededAt != nil {
		t.Errorf("confirmation.SucceededAt = %v, want nil (作成直後は未確認)", confirmation.SucceededAt)
	}
	if confirmation.CreatedAt.IsZero() {
		t.Error("confirmation.CreatedAt は DB 既定値で設定されるはず")
	}
	if confirmation.UpdatedAt.IsZero() {
		t.Error("confirmation.UpdatedAt は DB 既定値で設定されるはず")
	}
}

// TestEmailConfirmationRepository_CreatePreservesEmailCase confirms the citext
// email column stores the address as given (a confirmation is keyed by the exact
// address the user typed), while still matching case-insensitively elsewhere.
//
// [Ja] TestEmailConfirmationRepository_CreatePreservesEmailCase は citext の email
// 列が与えたとおりにアドレスを保存することを確認する (確認はユーザーが入力した正確な
// アドレスをキーとする)。citext は他の照合では大文字小文字を無視する。
func TestEmailConfirmationRepository_CreatePreservesEmailCase(t *testing.T) {
	t.Parallel()

	repo, ctx := newEmailConfirmationRepo(t)

	confirmation, err := repo.Create(ctx, repository.CreateEmailConfirmationInput{
		Email: "Mixed.Case@Example.com",
		Event: model.EmailConfirmationEventSignUp,
		Code:  "654321",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if confirmation.Email != "Mixed.Case@Example.com" {
		t.Errorf("confirmation.Email = %q, want %q", confirmation.Email, "Mixed.Case@Example.com")
	}
}

// TestEmailConfirmationRepository_FindActiveByID covers the "active" filter:
// a freshly issued confirmation is returned, while an unknown id, an already
// succeeded confirmation, and one issued outside the 15-minute window each yield
// (nil, nil).
//
// [Ja] TestEmailConfirmationRepository_FindActiveByID は "active" フィルタを網羅する。
// 発行直後の確認は返り、未知の id・確認済み・15 分のウィンドウ外で発行された確認は
// いずれも (nil, nil) になる。
func TestEmailConfirmationRepository_FindActiveByID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	t.Run("発行直後の確認は active として返る", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, tx).WithCode("123456").Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got == nil {
			t.Fatal("発行直後の確認は返るはず (nil が返った)")
		}
		if got.ID != id {
			t.Errorf("got.ID = %v, want %v", got.ID, id)
		}
		if got.Code != "123456" {
			t.Errorf("got.Code = %q, want %q", got.Code, "123456")
		}
	})

	t.Run("未知の id は nil", func(t *testing.T) {
		got, err := repo.FindActiveByID(ctx, model.EmailConfirmationID(uuid.New()))
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("未知の id は nil を返すはず: %+v", got)
		}
	})

	t.Run("確認済み (succeeded_at あり) は active でない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, tx).Build()
		if err := repo.Succeed(ctx, id); err != nil {
			t.Fatalf("Succeed() error = %v", err)
		}

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("確認済みは active でないはず: %+v", got)
		}
	})

	t.Run("有効期限切れ (started_at が 15 分より前) は active でない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, tx).
			WithStartedAt(time.Now().Add(-16 * time.Minute)).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("期限切れは active でないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限に達した確認は active でない", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, tx).
			WithFailedAttemptsCount(5).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got != nil {
			t.Errorf("試行回数超過は active でないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限未満の確認は active として返り回数も読める", func(t *testing.T) {
		id := testutil.NewEmailConfirmationBuilder(t, tx).
			WithFailedAttemptsCount(4).
			Build()

		got, err := repo.FindActiveByID(ctx, id)
		if err != nil {
			t.Fatalf("FindActiveByID() error = %v", err)
		}
		if got == nil {
			t.Fatal("上限未満の確認は active として返るはず (nil が返った)")
		}
		if got.FailedAttemptsCount != 4 {
			t.Errorf("got.FailedAttemptsCount = %d, want 4", got.FailedAttemptsCount)
		}
	})
}

// TestEmailConfirmationRepository_IncrementFailedAttempts verifies that each call
// bumps failed_attempts_count by one, reading the count back through
// FindActiveByID (which still returns the row while it is under the limit).
//
// [Ja] TestEmailConfirmationRepository_IncrementFailedAttempts は、各呼び出しが
// failed_attempts_count を 1 ずつ増やすことを検証する。回数は FindActiveByID (上限未満の
// 間は行を返す) 経由で読み戻す。
func TestEmailConfirmationRepository_IncrementFailedAttempts(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	id := testutil.NewEmailConfirmationBuilder(t, tx).Build()

	if err := repo.IncrementFailedAttempts(ctx, id); err != nil {
		t.Fatalf("IncrementFailedAttempts() error = %v", err)
	}
	got, err := repo.FindActiveByID(ctx, id)
	if err != nil {
		t.Fatalf("FindActiveByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("1 回のインクリメント後はまだ active のはず")
	}
	if got.FailedAttemptsCount != 1 {
		t.Errorf("1 回のインクリメント後の FailedAttemptsCount = %d, want 1", got.FailedAttemptsCount)
	}

	if err := repo.IncrementFailedAttempts(ctx, id); err != nil {
		t.Fatalf("IncrementFailedAttempts() error = %v", err)
	}
	got, err = repo.FindActiveByID(ctx, id)
	if err != nil {
		t.Fatalf("FindActiveByID() error = %v", err)
	}
	if got == nil {
		t.Fatal("2 回のインクリメント後はまだ active のはず")
	}
	if got.FailedAttemptsCount != 2 {
		t.Errorf("2 回のインクリメント後の FailedAttemptsCount = %d, want 2", got.FailedAttemptsCount)
	}
}

// TestEmailConfirmationRepository_Succeed verifies Succeed stamps succeeded_at on
// the row, reading it back directly to confirm it is no longer NULL.
//
// [Ja] TestEmailConfirmationRepository_Succeed は Succeed が行の succeeded_at を打刻する
// ことを検証する。直接読み戻して NULL でなくなったことを確認する。
func TestEmailConfirmationRepository_Succeed(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	id := testutil.NewEmailConfirmationBuilder(t, tx).Build()

	if err := repo.Succeed(ctx, id); err != nil {
		t.Fatalf("Succeed() error = %v", err)
	}

	var succeededAt *time.Time
	err := tx.QueryRow(ctx, `SELECT succeeded_at FROM email_confirmations WHERE id = $1`, uuid.UUID(id)).Scan(&succeededAt)
	if err != nil {
		t.Fatalf("succeeded_at の読み戻しに失敗: %v", err)
	}
	if succeededAt == nil {
		t.Error("Succeed() は succeeded_at を打刻するはず (NULL のまま)")
	}
}

// TestEmailConfirmationRepository_CreateEmailChange verifies an email-change
// confirmation is inserted tied to the requesting user, with the event fixed to
// email_change and the new address stored in email.
//
// [Ja] TestEmailConfirmationRepository_CreateEmailChange は、メール変更の確認が申請した
// ユーザーに紐付いて挿入され、event が email_change に固定され、新しいアドレスが email に
// 保存されることを検証する。
func TestEmailConfirmationRepository_CreateEmailChange(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	userID := testutil.NewUserBuilder(t, tx).Build()

	confirmation, err := repo.CreateEmailChange(ctx, repository.CreateEmailChangeInput{
		UserID: userID,
		Email:  "new-address@example.com",
		Code:   "123456",
	})
	if err != nil {
		t.Fatalf("CreateEmailChange() error = %v", err)
	}

	if confirmation.ID == (model.EmailConfirmationID{}) {
		t.Error("CreateEmailChange() confirmation.ID は DB 採番で空でないはず")
	}
	if confirmation.UserID == nil {
		t.Fatal("confirmation.UserID は設定されるはず (nil が返った)")
	}
	if *confirmation.UserID != userID {
		t.Errorf("*confirmation.UserID = %v, want %v", *confirmation.UserID, userID)
	}
	if confirmation.Email != "new-address@example.com" {
		t.Errorf("confirmation.Email = %q, want %q", confirmation.Email, "new-address@example.com")
	}
	if confirmation.Event != model.EmailConfirmationEventEmailChange {
		t.Errorf("confirmation.Event = %q, want %q", confirmation.Event, model.EmailConfirmationEventEmailChange)
	}
	if confirmation.Code != "123456" {
		t.Errorf("confirmation.Code = %q, want %q", confirmation.Code, "123456")
	}
	if confirmation.SucceededAt != nil {
		t.Errorf("confirmation.SucceededAt = %v, want nil (作成直後は未確認)", confirmation.SucceededAt)
	}
	if confirmation.StartedAt.IsZero() {
		t.Error("confirmation.StartedAt は DB 既定値で設定されるはず")
	}
}

// TestEmailConfirmationRepository_FindActiveEmailChangeByUserID covers the
// by-user "active" filter for email-change confirmations: a freshly issued one is
// returned, while a user with none, a sign-up confirmation (wrong event), an
// already succeeded one, an expired one, and an attempt-exhausted one each yield
// (nil, nil).
//
// [Ja] TestEmailConfirmationRepository_FindActiveEmailChangeByUserID はメール変更確認の
// ユーザー単位 "active" フィルタを網羅する。発行直後のものは返り、保留中の無いユーザー・
// サインアップ確認 (event 違い)・確認済み・期限切れ・試行超過はいずれも (nil, nil) になる。
func TestEmailConfirmationRepository_FindActiveEmailChangeByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	t.Run("発行直後のメール変更確認は active として返る", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		id := testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithCode("123456").
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got == nil {
			t.Fatal("発行直後の確認は返るはず (nil が返った)")
		}
		if got.ID != id {
			t.Errorf("got.ID = %v, want %v", got.ID, id)
		}
		if got.UserID == nil || *got.UserID != userID {
			t.Errorf("got.UserID = %v, want %v", got.UserID, userID)
		}
		if got.Code != "123456" {
			t.Errorf("got.Code = %q, want %q", got.Code, "123456")
		}
	})

	t.Run("保留中の確認が無いユーザーは nil", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("保留中の無いユーザーは nil を返すはず: %+v", got)
		}
	})

	t.Run("サインアップ確認 (event 違い) は返さない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		// A sign_up confirmation must not match the email_change lookup even when
		// it carries a user_id.
		//
		// [Ja] user_id を紐付けても event が sign_up の確認は email_change の
		// ルックアップにヒットしない。
		testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventSignUp).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("event が sign_up の確認は email_change として返らないはず: %+v", got)
		}
	})

	t.Run("確認済み (succeeded_at あり) は active でない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		id := testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()
		if err := repo.Succeed(ctx, id); err != nil {
			t.Fatalf("Succeed() error = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("確認済みは active でないはず: %+v", got)
		}
	})

	t.Run("有効期限切れ (started_at が 15 分より前) は active でない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithStartedAt(time.Now().Add(-16 * time.Minute)).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("期限切れは active でないはず: %+v", got)
		}
	})

	t.Run("試行回数が上限に達した確認は active でない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			WithFailedAttemptsCount(5).
			Build()

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("試行回数超過は active でないはず: %+v", got)
		}
	})
}

// TestEmailConfirmationRepository_DeleteUnusedEmailChangesByUserID verifies the
// delete keeps at most one pending confirmation per user: it removes a user's
// not-yet-succeeded email-change confirmations, leaves a succeeded one as a
// record, does not touch another user's confirmation, and is a no-op when there
// is nothing pending.
//
// [Ja] TestEmailConfirmationRepository_DeleteUnusedEmailChangesByUserID は、削除が
// ユーザーごとに保留中を高々 1 件に保つことを検証する。ユーザーの未確認のメール変更確認を
// 削除し、確認済みは記録として残し、他ユーザーの確認には触れず、保留中が無ければ no-op に
// なる。
func TestEmailConfirmationRepository_DeleteUnusedEmailChangesByUserID(t *testing.T) {
	t.Parallel()

	db, tx := testutil.SetupTx(t)
	repo := repository.NewEmailConfirmationRepository(query.New(db)).WithTx(tx)
	ctx := context.Background()

	t.Run("未確認のメール変更確認を削除する", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID() error = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, userID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got != nil {
			t.Errorf("削除後は保留中の確認が無いはず: %+v", got)
		}
	})

	t.Run("確認済みのものは残す", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		succeededID := testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(userID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()
		if err := repo.Succeed(ctx, succeededID); err != nil {
			t.Fatalf("Succeed() error = %v", err)
		}

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID() error = %v", err)
		}

		// Confirm the succeeded row is still present by reading it back directly.
		//
		// [Ja] 確認済みの行が残っていることを直接確認する。
		var count int
		if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM email_confirmations WHERE id = $1`, uuid.UUID(succeededID)).Scan(&count); err != nil {
			t.Fatalf("確認済み行の件数取得に失敗: %v", err)
		}
		if count != 1 {
			t.Errorf("確認済みの行は削除されないはず: count = %d, want 1", count)
		}
	})

	t.Run("他ユーザーの確認は削除しない", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()
		otherUserID := testutil.NewUserBuilder(t, tx).Build()
		testutil.NewEmailConfirmationBuilder(t, tx).
			WithUserID(otherUserID).
			WithEvent(model.EmailConfirmationEventEmailChange).
			Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("DeleteUnusedEmailChangesByUserID() error = %v", err)
		}

		got, err := repo.FindActiveEmailChangeByUserID(ctx, otherUserID)
		if err != nil {
			t.Fatalf("FindActiveEmailChangeByUserID() error = %v", err)
		}
		if got == nil {
			t.Error("他ユーザーの保留中確認は残るはず (nil が返った)")
		}
	})

	t.Run("保留中が無くてもエラーにならない (no-op)", func(t *testing.T) {
		userID := testutil.NewUserBuilder(t, tx).Build()

		if err := repo.DeleteUnusedEmailChangesByUserID(ctx, userID); err != nil {
			t.Fatalf("保留中が無いときの DeleteUnusedEmailChangesByUserID() error = %v", err)
		}
	})
}
