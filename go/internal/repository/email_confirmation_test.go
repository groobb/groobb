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
