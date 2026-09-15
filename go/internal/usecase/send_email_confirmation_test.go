package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/groobb/groobb/go/internal/model"
	"github.com/groobb/groobb/go/internal/usecase"
)

// fakeConfirmationSenderは最後のSend呼び出しの引数を記録し、任意で指定した
// エラーを返す。UseCaseが何を渡したか、失敗をどう伝搬するかをテストで検証するため。
type fakeConfirmationSender struct {
	called     bool
	to         string
	code       string
	locale     model.Locale
	returnErr  error
	callsCount int
}

func (f *fakeConfirmationSender) Send(_ context.Context, to, code string, locale model.Locale) error {
	f.called = true
	f.callsCount++
	f.to = to
	f.code = code
	f.locale = locale
	return f.returnErr
}

func TestSendEmailConfirmationUsecase_Execute(t *testing.T) {
	t.Parallel()

	t.Run("送信内容をsenderにそのまま渡す", func(t *testing.T) {
		t.Parallel()

		fake := &fakeConfirmationSender{}
		uc := usecase.NewSendEmailConfirmationUsecase(fake)

		err := uc.Execute(context.Background(), usecase.SendEmailConfirmationInput{
			Email:  "user@example.dev",
			Code:   "246813",
			Locale: "ja",
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
		if fake.code != "246813" {
			t.Errorf("code = %q、期待値 = %q", fake.code, "246813")
		}
		if fake.locale != "ja" {
			t.Errorf("locale = %q、期待値 = %q", fake.locale, "ja")
		}
	})

	t.Run("senderの失敗をそのまま返す", func(t *testing.T) {
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
			t.Errorf("Execute()のエラー = %v、期待値 = %v", err, wantErr)
		}
	})
}
