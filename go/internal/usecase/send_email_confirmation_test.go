package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/usecase"
)

// fakeConfirmationSender records the arguments of the last Send call and
// optionally returns a preset error, so tests can assert what the UseCase
// forwarded and how it propagates failures.
//
// [Ja] fakeConfirmationSender は最後の Send 呼び出しの引数を記録し、任意で指定した
// エラーを返す。UseCase が何を渡したか、失敗をどう伝搬するかをテストで検証するため。
type fakeConfirmationSender struct {
	called     bool
	to         string
	code       string
	locale     string
	returnErr  error
	callsCount int
}

func (f *fakeConfirmationSender) Send(_ context.Context, to, code, locale string) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.code = code
	f.locale = locale
	return f.returnErr
}

func TestSendEmailConfirmationUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容を sender にそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakeConfirmationSender{}
		uc := usecase.NewSendEmailConfirmationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailConfirmationInput{
			Email:  "user@example.dev",
			Code:   "246813",
			Locale: "ja",
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
		if fake.code != "246813" {
			t.Errorf("code = %q, want %q", fake.code, "246813")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q, want %q", fake.locale, "ja")
		}
	})

	t.Run("sender の失敗をそのまま返す", func(t *testing.T) {
		t.Parallel()

		wantErr := errors.New("送信失敗")
		fake := &fakeConfirmationSender{returnErr: wantErr}
		uc := usecase.NewSendEmailConfirmationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailConfirmationInput{
			Email:  "user@example.dev",
			Code:   "000000",
			Locale: "en",
		})
		if !errors.Is(err, wantErr) {
			t.Errorf("Execute() error = %v, want %v", err, wantErr)
		}
	})
}
