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
