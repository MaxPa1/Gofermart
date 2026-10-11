package password

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestHasher(t *testing.T) *Hasher {
	t.Helper()
	h, err := NewHasher(bcrypt.MinCost)
	require.NoError(t, err)
	return h
}

func TestNewHasher(t *testing.T) {
	tests := []struct {
		name    string
		cost    int
		wantErr bool
	}{
		{name: "min cost", cost: bcrypt.MinCost},
		{name: "default cost", cost: bcrypt.DefaultCost},
		{name: "max cost", cost: bcrypt.MaxCost},
		{name: "below min", cost: bcrypt.MinCost - 1, wantErr: true},
		{name: "above max", cost: bcrypt.MaxCost + 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := NewHasher(tt.cost)
			if tt.wantErr {
				require.ErrorIs(t, err, ErrInvalidCost)
				assert.Nil(t, h)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, h)
		})
	}
}

func TestHasher_Hash(t *testing.T) {
	h := newTestHasher(t)

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "regular password", password: "s3cret-Passw0rd"},
		{name: "unicode password", password: "пароль"},
		{name: "exactly max length", password: strings.Repeat("a", MaxLength)},
		{name: "longer than max", password: strings.Repeat("a", MaxLength+1), wantErr: ErrTooLong},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash, err := h.Hash(tt.password)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, hash)
				return
			}
			require.NoError(t, err)
			assert.NotEqual(t, tt.password, hash, "hash must not equal the plain password")

			cost, err := bcrypt.Cost([]byte(hash))
			require.NoError(t, err)
			assert.Equal(t, bcrypt.MinCost, cost)
		})
	}
}

func TestHasher_Hash_UsesRandomSalt(t *testing.T) {
	h := newTestHasher(t)

	first, err := h.Hash("same-password")
	require.NoError(t, err)
	second, err := h.Hash("same-password")
	require.NoError(t, err)

	assert.NotEqual(t, first, second, "hashes of the same password must differ because of the salt")
}

func TestHasher_Compare(t *testing.T) {
	h := newTestHasher(t)

	const plain = "s3cret-Passw0rd"
	hash, err := h.Hash(plain)
	require.NoError(t, err)

	maxLen := strings.Repeat("a", MaxLength)
	maxLenHash, err := h.Hash(maxLen)
	require.NoError(t, err)

	tests := []struct {
		name      string
		hash      string
		password  string
		wantMatch bool
		wantErr   bool
	}{
		{name: "correct password", hash: hash, password: plain, wantMatch: true},
		{name: "wrong password", hash: hash, password: "wrong"},
		{name: "empty password", hash: hash, password: ""},
		{name: "case differs", hash: hash, password: strings.ToUpper(plain)},
		{name: "max length matches", hash: maxLenHash, password: maxLen, wantMatch: true},
		{
			name:     "longer password with same 72-byte prefix does not match",
			hash:     maxLenHash,
			password: maxLen + "extra",
		},
		{name: "malformed hash", hash: "not-a-bcrypt-hash", password: plain, wantErr: true},
		{name: "empty hash", hash: "", password: plain, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := h.Compare(tt.hash, tt.password)
			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, match)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatch, match)
		})
	}
}
