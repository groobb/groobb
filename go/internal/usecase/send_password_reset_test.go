package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
)

// fakePasswordResetSenderは最後のSend呼び出しの引数を記録し、任意で指定した
// エラーを返す。UseCaseが何を渡したか、失敗をどう伝搬するかをテストで検証するため。
type fakePasswordResetSender struct {
	called     bool
	to         string
	resetURL   string
	locale     model.Locale
	returnErr  error
	callsCount int
}

func (f *fakePasswordResetSender) Send(_ context.Context, to, resetURL string, locale model.Locale) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.resetURL = resetURL
	f.locale = locale
	return f.returnErr
}

func TestSendPasswordResetUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容をsenderにそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakePasswordResetSender{}
		uc := usecase.NewSendPasswordResetUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendPasswordResetInput{
			Email:    "user@example.dev",
			ResetURL: "https://groobb.example.dev/password/edit?token=abc",
			Locale:   "ja",
		})
		if err != nil {
			t.Fatalf("Execute()のエラー = %v", err)
		}

		if !fake.called {
			t.Fatal("sender.Sendが呼ばれていません")
		}
		if fake.callsCount != 1 {
			t.Errorf("Sendの呼び出し回数 = %d、期待値 = 1", fake.callsCount)
		}
		if fake.to != "user@example.dev" {
			t.Errorf("to = %q、期待値 = %q", fake.to, "user@example.dev")
		}
		if fake.resetURL != "https://groobb.example.dev/password/edit?token=abc" {
			t.Errorf("resetURL = %q、期待値 = %q", fake.resetURL, "https://groobb.example.dev/password/edit?token=abc")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q、期待値 = %q", fake.locale, "ja")
		}
	})

	t.Run("senderの失敗をそのまま返す", func(t *testing.T) {
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
			t.Errorf("Execute()のエラー = %v、期待値 = %v", err, wantErr)
		}
	})
}
