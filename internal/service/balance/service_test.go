package balance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/MaxPa1/Gofermart/internal/model"
)

const (
	testUserID  = int64(42)
	validNumber = "2377225624"
)

var errInternal = errors.New("boom")

func newTestService(t *testing.T) (*Service, *MockRepository) {
	t.Helper()
	repo := NewMockRepository(gomock.NewController(t))
	return NewService(repo), repo
}

func TestService_Balance(t *testing.T) {
	t.Run("balance", func(t *testing.T) {
		svc, repo := newTestService(t)
		want := model.Balance{Current: 50050, Withdrawn: 4200}
		repo.EXPECT().Get(gomock.Any(), testUserID).Return(want, nil)

		got, err := svc.Balance(context.Background(), testUserID)
		require.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("repository error", func(t *testing.T) {
		svc, repo := newTestService(t)
		repo.EXPECT().Get(gomock.Any(), testUserID).Return(model.Balance{}, errInternal)

		_, err := svc.Balance(context.Background(), testUserID)
		require.ErrorIs(t, err, errInternal)
	})
}

func TestService_Withdraw(t *testing.T) {
	tests := []struct {
		name      string
		number    string
		sum       model.Amount
		callsRepo bool
		repoErr   error
		wantErr   error
	}{
		{name: "success", number: validNumber, sum: 75100, callsRepo: true},
		{name: "fractional sum", number: validNumber, sum: 1, callsRepo: true},
		{name: "insufficient funds", number: validNumber, sum: 75100, callsRepo: true, repoErr: model.ErrInsufficientFunds, wantErr: model.ErrInsufficientFunds},
		{name: "repository error", number: validNumber, sum: 75100, callsRepo: true, repoErr: errInternal, wantErr: errInternal},

		{name: "empty number", number: "", sum: 75100, wantErr: model.ErrInvalidOrderNumber},
		{name: "wrong check digit", number: "2377225625", sum: 75100, wantErr: model.ErrInvalidOrderNumber},
		{name: "not digits", number: "abc", sum: 75100, wantErr: model.ErrInvalidOrderNumber},
		{name: "zero sum", number: validNumber, sum: 0, wantErr: model.ErrInvalidInput},
		{name: "negative sum", number: validNumber, sum: -100, wantErr: model.ErrInvalidInput},
		{name: "invalid number and sum", number: "abc", sum: 0, wantErr: model.ErrInvalidOrderNumber},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService(t)
			if tt.callsRepo {
				repo.EXPECT().Withdraw(gomock.Any(), testUserID, tt.number, tt.sum).Return(tt.repoErr)
			}

			err := svc.Withdraw(context.Background(), testUserID, tt.number, tt.sum)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestService_Withdrawals(t *testing.T) {
	withdrawals := []model.Withdrawal{
		{UserID: testUserID, OrderNumber: validNumber, Sum: 50000, ProcessedAt: time.Now()},
	}

	tests := []struct {
		name    string
		repoOut []model.Withdrawal
		repoErr error
		wantErr error
	}{
		{name: "withdrawals", repoOut: withdrawals},
		{name: "no withdrawals", repoOut: []model.Withdrawal{}},
		{name: "repository error", repoErr: errInternal, wantErr: errInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService(t)
			repo.EXPECT().ListWithdrawals(gomock.Any(), testUserID).Return(tt.repoOut, tt.repoErr)

			got, err := svc.Withdrawals(context.Background(), testUserID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.repoOut, got)
		})
	}
}
