package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MaxPa1/Gofermart/internal/model"
)

type Status string

const (
	StatusRegistered Status = "REGISTERED"

	StatusInvalid Status = "INVALID"

	StatusProcessing Status = "PROCESSING"

	StatusProcessed Status = "PROCESSED"
)

const (
	defaultRetryAfter = 60 * time.Second

	maxResponseSize = 64 << 10
)

var ErrOrderNotRegistered = errors.New("order is not registered in accrual system")

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("accrual rate limit exceeded, retry after %s", e.RetryAfter)
}

type Result struct {
	Order  string
	Status Status

	Accrual model.Amount
}

type Client struct {
	baseURL *url.URL
	http    *http.Client

	now func() time.Time
}

func New(address string, timeout time.Duration) (*Client, error) {
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	u, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("parse accrual address: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("invalid accrual address %q: want http(s)://host[:port]", address)
	}

	return &Client{
		baseURL: u,

		http: &http.Client{Timeout: timeout},
		now:  time.Now,
	}, nil
}

type orderResponse struct {
	Order   string      `json:"order"`
	Status  Status      `json:"status"`
	Accrual json.Number `json:"accrual"`
}

func (c *Client) GetOrder(ctx context.Context, number string) (Result, error) {
	endpoint := c.baseURL.JoinPath("api", "orders", number)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("request accrual: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseSize))
		_ = resp.Body.Close()
	}()

	switch resp.StatusCode {
	case http.StatusOK:
		return decodeOrder(resp.Body, number)
	case http.StatusNoContent:
		return Result{}, ErrOrderNotRegistered
	case http.StatusTooManyRequests:
		return Result{}, &RateLimitError{RetryAfter: c.retryAfter(resp.Header.Get("Retry-After"))}
	default:
		return Result{}, fmt.Errorf("unexpected accrual response status %d", resp.StatusCode)
	}
}

func decodeOrder(body io.Reader, number string) (Result, error) {
	var r orderResponse
	if err := json.NewDecoder(io.LimitReader(body, maxResponseSize)).Decode(&r); err != nil {
		return Result{}, fmt.Errorf("decode accrual response: %w", err)
	}

	switch r.Status {
	case StatusRegistered, StatusInvalid, StatusProcessing, StatusProcessed:
	default:
		return Result{}, fmt.Errorf("unknown accrual status %q", r.Status)
	}
	if r.Order != number {
		return Result{}, fmt.Errorf("accrual responded for order %q, requested %q", r.Order, number)
	}

	res := Result{Order: r.Order, Status: r.Status}

	if r.Accrual != "" {
		amount, err := model.ParseAmountRounded(r.Accrual.String())
		if err != nil {
			return Result{}, fmt.Errorf("parse accrual %q: %w", r.Accrual, err)
		}
		if amount < 0 {
			return Result{}, fmt.Errorf("negative accrual %s", amount)
		}
		res.Accrual = amount
	}
	return res, nil
}

func (c *Client) retryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := t.Sub(c.now()); d > 0 {
			return d
		}

		return 0
	}
	return defaultRetryAfter
}
