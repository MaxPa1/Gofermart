// Package accrual — HTTP-клиент внешней системы расчёта начислений баллов (accrual).
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

// Status — статус расчёта начисления в терминах системы accrual.
type Status string

const (
	// StatusRegistered — заказ зарегистрирован, но начисление ещё не рассчитано.
	StatusRegistered Status = "REGISTERED"

	// StatusInvalid — заказ не принят к расчёту, начисления не будет.
	StatusInvalid Status = "INVALID"

	// StatusProcessing — начисление рассчитывается.
	StatusProcessing Status = "PROCESSING"

	// StatusProcessed — расчёт окончен, начисление известно.
	StatusProcessed Status = "PROCESSED"
)

const (
	defaultRetryAfter = 60 * time.Second

	maxResponseSize = 64 << 10
)

// ErrOrderNotRegistered возвращается Client.GetOrder, если accrual ответил 204 No Content: заказ в
// системе расчёта не зарегистрирован.
var ErrOrderNotRegistered = errors.New("order is not registered in accrual system")

// RateLimitError возвращается Client.GetOrder, если accrual ответил 429 Too Many Requests. Все запросы
// к accrual следует приостановить на RetryAfter.
type RateLimitError struct {
	// RetryAfter — сколько ждать до следующего запроса. Если заголовок отсутствует или некорректен — 60 секунд.
	RetryAfter time.Duration
}

// Error возвращает описание ошибки с длительностью паузы.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("accrual rate limit exceeded, retry after %s", e.RetryAfter)
}

// Result — результат расчёта начисления по заказу.
type Result struct {
	// Order — номер заказа.
	Order string
	// Status — статус расчёта.
	Status Status

	// Accrual — начисленные баллы; ненулевое значение бывает только в статусе StatusProcessed.
	Accrual model.Amount
}

// Client — клиент HTTP API системы accrual.
type Client struct {
	baseURL *url.URL
	http    *http.Client
}

// New создаёт клиент accrual по адресу address с ограничением timeout на весь запрос
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
	}, nil
}

type orderResponse struct {
	Order   string      `json:"order"`
	Status  Status      `json:"status"`
	Accrual json.Number `json:"accrual"`
}

// GetOrder запрашивает у accrual результат расчёта начисления по заказу number. Возвращает Result при
// ответе 200, ErrOrderNotRegistered при 204, *RateLimitError при 429 и ошибку при любом другом коде
// ответа, сетевом сбое, отмене ctx или некорректном теле ответа.
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
		return Result{}, &RateLimitError{RetryAfter: retryAfter(resp.Header.Get("Retry-After"))}
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

func retryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if secs, err := strconv.Atoi(header); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(header); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}

		return 0
	}
	return defaultRetryAfter
}
