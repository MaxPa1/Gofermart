package handler

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MaxPa1/Gofermart/internal/model"
)

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
	require.ErrorIs(t, err, model.ErrInvalidAmount)
}
