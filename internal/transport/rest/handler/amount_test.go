package handler

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaxPa1/Gofermart/internal/model"
)

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		amount model.Amount
		want   string
	}{
		{amount: 0, want: "0"},
		{amount: 50000, want: "500"},
		{amount: 50050, want: "500.5"},
		{amount: 50005, want: "500.05"},
		{amount: 72998, want: "729.98"},
		{amount: 1, want: "0.01"},
		{amount: 10, want: "0.1"},
		{amount: -150, want: "-1.5"},
		{amount: -5, want: "-0.05"},
		{amount: math.MaxInt64, want: "92233720368547758.07"},
		{amount: math.MinInt64, want: "-92233720368547758.08"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, formatAmount(tt.amount))
		})
	}
}

func TestJSONAmount_MarshalJSON(t *testing.T) {
	type payload struct {
		Amount jsonAmount `json:"amount,omitempty"`
	}

	got, err := json.Marshal(payload{Amount: 50050})
	require.NoError(t, err)
	assert.JSONEq(t, `{"amount": 500.5}`, string(got))

	got, err = json.Marshal(payload{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(got))
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		in      string
		want    model.Amount
		wantErr bool
	}{
		{in: "751", want: 75100},
		{in: "500.5", want: 50050},
		{in: "500.50", want: 50050},
		{in: "500.500", want: 50050},
		{in: "0.05", want: 5},
		{in: "0.01", want: 1},
		{in: "0", want: 0},
		{in: "729.98", want: 72998},
		{in: "-1.5", want: -150},
		{in: "92233720368547758.07", want: math.MaxInt64},

		{in: "1.234", wantErr: true},
		{in: "0.001", wantErr: true},
		{in: "7.51e2", wantErr: true},
		{in: "1E3", wantErr: true},
		{in: `"751"`, wantErr: true},
		{in: "", wantErr: true},
		{in: "-", wantErr: true},
		{in: ".5", wantErr: true},
		{in: "5.", want: 500},
		{in: "abc", wantErr: true},
		{in: "true", wantErr: true},
		{in: "92233720368547758.08", wantErr: true},
		{in: "100000000000000000000", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseAmount(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, errInvalidAmount)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseAmount_RoundTrip(t *testing.T) {
	for _, a := range []model.Amount{0, 1, 10, 99, 100, 50050, 72998, 75100, -150, math.MaxInt64, math.MinInt64 + 1} {
		got, err := parseAmount(formatAmount(a))
		require.NoError(t, err)
		assert.Equal(t, a, got)
	}
}

func TestJSONAmount_UnmarshalJSON(t *testing.T) {
	var p struct {
		Sum jsonAmount `json:"sum"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"sum": 500.5}`), &p))
	assert.Equal(t, jsonAmount(50050), p.Sum)

	p.Sum = 0
	require.NoError(t, json.Unmarshal([]byte(`{"sum": null}`), &p))
	assert.Equal(t, jsonAmount(0), p.Sum)

	err := json.Unmarshal([]byte(`{"sum": 1.234}`), &p)
	require.ErrorIs(t, err, errInvalidAmount)
}
