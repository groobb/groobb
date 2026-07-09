package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/usecase"
)

// fakeEmailChangeNotificationSender records the arguments of the last Send call
// and optionally returns a preset error, so tests can assert what the UseCase
// forwarded and how it propagates failures.
//
// [Ja] fakeEmailChangeNotificationSender は最後の Send 呼び出しの引数を記録し、任意で
// 指定したエラーを返す。UseCase が何を渡したか、失敗をどう伝搬するかをテストで検証する
// ため。
type fakeEmailChangeNotificationSender struct {
	called     bool
	to         string
	newEmail   string
	locale     string
	returnErr  error
	callsCount int
}

func (f *fakeEmailChangeNotificationSender) Send(_ context.Context, to, newEmail, locale string) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.newEmail = newEmail
	f.locale = locale
	return f.returnErr
}

func TestSendEmailChangeNotificationUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容を sender にそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakeEmailChangeNotificationSender{}
		uc := usecase.NewSendEmailChangeNotificationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailChangeNotificationInput{
			Email:    "old@example.dev",
			NewEmail: "new@example.dev",
			Locale:   "ja",
		})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !fake.called {
			t.Fatal("sender.Send が呼ばれていません")
		}
		if fake.callsCount != 1 {
			t.Errorf("Send の呼び出し回数 = %d, want 1", fake.callsCount)
		}
		if fake.to != "old@example.dev" {
			t.Errorf("to = %q, want %q", fake.to, "old@example.dev")
		}
		if fake.newEmail != "new@example.dev" {
			t.Errorf("newEmail = %q, want %q", fake.newEmail, "new@example.dev")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q, want %q", fake.locale, "ja")
		}
	})

	t.Run("sender の失敗をそのまま返す", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("送信失敗")
		fake := &fakeEmailChangeNotificationSender{returnErr: wantErr}
		uc := usecase.NewSendEmailChangeNotificationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailChangeNotificationInput{
			Email:    "old@example.dev",
			NewEmail: "new@example.dev",
			Locale:   "en",
		})
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
	})
}
