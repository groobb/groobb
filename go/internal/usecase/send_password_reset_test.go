package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/usecase"
)

// fakePasswordResetSender records the arguments of the last Send call and
// optionally returns a preset error, so tests can assert what the UseCase
// forwarded and how it propagates failures.
//
// [Ja] fakePasswordResetSender は最後の Send 呼び出しの引数を記録し、任意で指定した
// エラーを返す。UseCase が何を渡したか、失敗をどう伝搬するかをテストで検証するため。
type fakePasswordResetSender struct {
	called     bool
	to         string
	resetURL   string
	locale     string
	returnErr  error
	callsCount int
}

func (f *fakePasswordResetSender) Send(_ context.Context, to, resetURL, locale string) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.resetURL = resetURL
	f.locale = locale
	return f.returnErr
}

func TestSendPasswordResetUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容を sender にそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakePasswordResetSender{}
		uc := usecase.NewSendPasswordResetUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{
			Email:    "user@example.dev",
			ResetURL: "https://groobb.example.dev/password/edit?token=abc",
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
		if fake.to != "user@example.dev" {
			t.Errorf("to = %q, want %q", fake.to, "user@example.dev")
		}
		if fake.resetURL != "https://groobb.example.dev/password/edit?token=abc" {
			t.Errorf("resetURL = %q, want %q", fake.resetURL, "https://groobb.example.dev/password/edit?token=abc")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q, want %q", fake.locale, "ja")
		}
	})

	t.Run("sender の失敗をそのまま返す", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("送信失敗")
		fake := &fakePasswordResetSender{returnErr: wantErr}
		uc := usecase.NewSendPasswordResetUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{
			Email:    "user@example.dev",
			ResetURL: "https://groobb.example.dev/password/edit?token=xyz",
			Locale:   "en",
		})
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
	})
}
