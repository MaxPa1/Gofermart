package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOrder = "12345678903"

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, time.Second)
	require.NoError(t, err)
	return c
}

func TestNew(t *testing.T) {
	tests := []struct {
		address string
		wantURL string
		wantErr bool
	}{
		{address: "http://localhost:8081", wantURL: "http://localhost:8081"},
		{address: "https://accrual.example.com", wantURL: "https://accrual.example.com"},
		{address: "localhost:8081", wantURL: "http://localhost:8081"},
		{address: "ftp://localhost:8081", wantErr: true},
		{address: "http://", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			c, err := New(tt.address, time.Second)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantURL, c.baseURL.String())
		})
	}
}

func TestClient_GetOrder(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Result
	}{
		{
			name: "processed with accrual",
			body: `{"order": "12345678903", "status": "PROCESSED", "accrual": 500}`,
			want: Result{Order: testOrder, Status: StatusProcessed, Accrual: 50000},
		},
		{
			name: "fractional accrual",
			body: `{"order": "12345678903", "status": "PROCESSED", "accrual": 729.98}`,
			want: Result{Order: testOrder, Status: StatusProcessed, Accrual: 72998},
		},
		{
			name: "accrual is rounded to hundredths",
			body: `{"order": "12345678903", "status": "PROCESSED", "accrual": 9.25875}`,
			want: Result{Order: testOrder, Status: StatusProcessed, Accrual: 926},
		},
		{
			name: "processed without accrual",
			body: `{"order": "12345678903", "status": "PROCESSED"}`,
			want: Result{Order: testOrder, Status: StatusProcessed},
		},
		{
			name: "accrual as numeric string",
			body: `{"order": "12345678903", "status": "PROCESSED", "accrual": "500"}`,
			want: Result{Order: testOrder, Status: StatusProcessed, Accrual: 50000},
		},
		{name: "registered", body: `{"order": "12345678903", "status": "REGISTERED"}`, want: Result{Order: testOrder, Status: StatusRegistered}},
		{name: "processing", body: `{"order": "12345678903", "status": "PROCESSING"}`, want: Result{Order: testOrder, Status: StatusProcessing}},
		{name: "invalid", body: `{"order": "12345678903", "status": "INVALID"}`, want: Result{Order: testOrder, Status: StatusInvalid}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/api/orders/"+testOrder, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			})

			got, err := c.GetOrder(context.Background(), testOrder)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestClient_GetOrder_InvalidResponse(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "malformed JSON", body: `{"order": `},
		{name: "unknown status", body: `{"order": "12345678903", "status": "DONE"}`},
		{name: "another order", body: `{"order": "79927398713", "status": "PROCESSED", "accrual": 500}`},
		{name: "negative accrual", body: `{"order": "12345678903", "status": "PROCESSED", "accrual": -1}`},
		{name: "accrual is not a number", body: `{"order": "12345678903", "status": "PROCESSED", "accrual": "abc"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			})

			_, err := c.GetOrder(context.Background(), testOrder)
			require.Error(t, err)
		})
	}
}

func TestClient_GetOrder_NotRegistered(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := c.GetOrder(context.Background(), testOrder)
	require.ErrorIs(t, err, ErrOrderNotRegistered)
}

func TestClient_GetOrder_RateLimit(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("No more than 10 requests per minute allowed"))
	})

	_, err := c.GetOrder(context.Background(), testOrder)
	var rl *RateLimitError
	require.ErrorAs(t, err, &rl)
	assert.Equal(t, 60*time.Second, rl.RetryAfter)
}

func TestRetryAfter(t *testing.T) {
	tests := []struct {
		name   string
		header func(now time.Time) string
		want   time.Duration
	}{
		{name: "seconds", header: func(time.Time) string { return "60" }, want: 60 * time.Second},
		{name: "zero seconds", header: func(time.Time) string { return "0" }, want: 0},
		{name: "spaces around seconds", header: func(time.Time) string { return " 5 " }, want: 5 * time.Second},
		{name: "http date", header: func(now time.Time) string { return now.Add(90 * time.Second).Format(http.TimeFormat) }, want: 90 * time.Second},
		{name: "date in the past", header: func(now time.Time) string { return now.Add(-time.Minute).Format(http.TimeFormat) }, want: 0},
		{name: "missing header", header: func(time.Time) string { return "" }, want: defaultRetryAfter},
		{name: "garbage", header: func(time.Time) string { return "soon" }, want: defaultRetryAfter},
		{name: "negative seconds", header: func(time.Time) string { return "-5" }, want: defaultRetryAfter},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				assert.Equal(t, tt.want, retryAfter(tt.header(time.Now().UTC())))
			})
		})
	}
}

func TestClient_GetOrder_ServerError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := c.GetOrder(context.Background(), testOrder)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrOrderNotRegistered)
	var rl *RateLimitError
	assert.False(t, errors.As(err, &rl), "500 must not be treated as rate limit")
}

func TestClient_GetOrder_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	t.Cleanup(srv.Close)

	c, err := New(srv.URL, 50*time.Millisecond)
	require.NoError(t, err)

	start := time.Now()
	_, err = c.GetOrder(context.Background(), testOrder)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 500*time.Millisecond, "request must be bounded by client timeout")
}

func TestClient_GetOrder_ContextCanceled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.GetOrder(ctx, testOrder)
	require.ErrorIs(t, err, context.Canceled)
}

func TestClient_GetOrder_LargeResponseIsLimited(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"order": "12345678903", "status": "` + strings.Repeat("A", 2*maxResponseSize) + `"}`))
	})

	_, err := c.GetOrder(context.Background(), testOrder)
	require.Error(t, err)
}
