package orders

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
	otherUserID = int64(7)
	validNumber = "12345678903"
)

var errInternal = errors.New("boom")

func newTestService(t *testing.T) (*Service, *MockOrderRepository) {
	t.Helper()
	repo := NewMockOrderRepository(gomock.NewController(t))
	return NewService(repo), repo
}

func TestService_Upload(t *testing.T) {
	tests := []struct {
		name    string
		number  string
		setup   func(repo *MockOrderRepository)
		wantErr error
	}{
		{
			name:   "new order",
			number: validNumber,
			setup: func(repo *MockOrderRepository) {
				repo.EXPECT().Create(gomock.Any(), testUserID, validNumber).Return(testUserID, true, nil)
			},
		},
		{
			name:   "already uploaded by same user",
			number: validNumber,
			setup: func(repo *MockOrderRepository) {
				repo.EXPECT().Create(gomock.Any(), testUserID, validNumber).Return(testUserID, false, nil)
			},
			wantErr: model.ErrOrderAlreadyUploaded,
		},
		{
			name:   "uploaded by another user",
			number: validNumber,
			setup: func(repo *MockOrderRepository) {
				repo.EXPECT().Create(gomock.Any(), testUserID, validNumber).Return(otherUserID, false, nil)
			},
			wantErr: model.ErrOrderOwnedByAnother,
		},
		{
			name:   "repository error",
			number: validNumber,
			setup: func(repo *MockOrderRepository) {
				repo.EXPECT().Create(gomock.Any(), testUserID, validNumber).Return(int64(0), false, errInternal)
			},
			wantErr: errInternal,
		},
		{name: "empty number", number: "", wantErr: model.ErrInvalidInput},
		{name: "wrong check digit", number: "12345678900", wantErr: model.ErrInvalidOrderNumber},
		{name: "not digits", number: "abc", wantErr: model.ErrInvalidOrderNumber},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService(t)
			if tt.setup != nil {
				tt.setup(repo)
			}

			err := svc.Upload(context.Background(), testUserID, tt.number)
			if tt.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestService_Upload_InternalErrorIsNotDomainError(t *testing.T) {
	svc, repo := newTestService(t)
	repo.EXPECT().Create(gomock.Any(), testUserID, validNumber).Return(int64(0), false, errInternal)

	err := svc.Upload(context.Background(), testUserID, validNumber)
	require.Error(t, err)
	for _, domainErr := range []error{
		model.ErrOrderAlreadyUploaded, model.ErrOrderOwnedByAnother,
		model.ErrInvalidOrderNumber, model.ErrInvalidInput,
	} {
		assert.NotErrorIs(t, err, domainErr)
	}
}

func TestService_List(t *testing.T) {
	orders := []model.Order{
		{Number: validNumber, UserID: testUserID, Status: model.OrderStatusProcessed, Accrual: 50000, UploadedAt: time.Now()},
		{Number: "79927398713", UserID: testUserID, Status: model.OrderStatusNew, UploadedAt: time.Now().Add(-time.Hour)},
	}

	tests := []struct {
		name    string
		repoOut []model.Order
		repoErr error
		want    []model.Order
		wantErr error
	}{
		{name: "orders", repoOut: orders, want: orders},
		{name: "no orders", repoOut: []model.Order{}, want: []model.Order{}},
		{name: "repository error", repoErr: errInternal, wantErr: errInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newTestService(t)
			repo.EXPECT().ListByUser(gomock.Any(), testUserID).Return(tt.repoOut, tt.repoErr)

			got, err := svc.List(context.Background(), testUserID)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
