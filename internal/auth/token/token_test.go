package token

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte(strings.Repeat("s", MinSecretLength))

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager(testSecret, time.Hour)
	require.NoError(t, err)
	return m
}

func signToken(t *testing.T, method jwt.SigningMethod, claims jwt.Claims, key any) string {
	t.Helper()
	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	require.NoError(t, err)
	return s
}

func validClaims() jwt.RegisteredClaims {
	now := time.Now()
	return jwt.RegisteredClaims{
		Subject:   "42",
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name    string
		secret  []byte
		ttl     time.Duration
		wantErr error
	}{
		{name: "valid", secret: testSecret, ttl: time.Hour},
		{name: "longer secret", secret: []byte(strings.Repeat("s", 64)), ttl: time.Minute},
		{name: "short secret", secret: []byte(strings.Repeat("s", MinSecretLength-1)), ttl: time.Hour, wantErr: ErrShortSecret},
		{name: "empty secret", secret: nil, ttl: time.Hour, wantErr: ErrShortSecret},
		{name: "zero ttl", secret: testSecret, ttl: 0, wantErr: ErrInvalidTTL},
		{name: "negative ttl", secret: testSecret, ttl: -time.Second, wantErr: ErrInvalidTTL},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewManager(tt.secret, tt.ttl)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, m)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, m)
		})
	}
}

func TestManager_GenerateParse_RoundTrip(t *testing.T) {
	m := newTestManager(t)

	for _, userID := range []int64{1, 42, 1<<63 - 1} {
		tok, err := m.Generate(userID)
		require.NoError(t, err)

		got, err := m.Parse(tok)
		require.NoError(t, err)
		assert.Equal(t, userID, got)
	}
}

func TestManager_Generate_Claims(t *testing.T) {
	const ttl = 15 * time.Minute
	m, err := NewManager(testSecret, ttl)
	require.NoError(t, err)

	tok, err := m.Generate(7)
	require.NoError(t, err)

	var claims jwt.RegisteredClaims
	parsed, err := jwt.ParseWithClaims(tok, &claims, func(*jwt.Token) (any, error) { return testSecret, nil })
	require.NoError(t, err)

	assert.Equal(t, jwt.SigningMethodHS256.Alg(), parsed.Method.Alg())
	assert.Equal(t, "7", claims.Subject)
	assert.Equal(t, issuer, claims.Issuer)
	require.NotNil(t, claims.IssuedAt)
	require.NotNil(t, claims.ExpiresAt)
	assert.Equal(t, ttl, claims.ExpiresAt.Sub(claims.IssuedAt.Time))
}

func TestManager_Generate_InvalidUserID(t *testing.T) {
	m := newTestManager(t)

	for _, userID := range []int64{0, -1} {
		tok, err := m.Generate(userID)
		require.ErrorIs(t, err, ErrInvalidUserID)
		assert.Empty(t, tok)
	}
}

func TestManager_Generate_SecretIsCopied(t *testing.T) {
	secret := []byte(strings.Repeat("k", MinSecretLength))
	m, err := NewManager(secret, time.Hour)
	require.NoError(t, err)

	tok, err := m.Generate(1)
	require.NoError(t, err)

	for i := range secret {
		secret[i] = 0
	}

	_, err = m.Parse(tok)
	require.NoError(t, err)
}

func TestManager_Parse_Invalid(t *testing.T) {
	m := newTestManager(t)

	valid, err := m.Generate(42)
	require.NoError(t, err)

	expired := validClaims()
	expired.IssuedAt = jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

	noExp := validClaims()
	noExp.ExpiresAt = nil

	futureIat := validClaims()
	futureIat.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour))

	foreignIssuer := validClaims()
	foreignIssuer.Issuer = "another-service"

	noIssuer := validClaims()
	noIssuer.Issuer = ""

	textSubject := validClaims()
	textSubject.Subject = "admin"

	emptySubject := validClaims()
	emptySubject.Subject = ""

	zeroSubject := validClaims()
	zeroSubject.Subject = "0"

	negativeSubject := validClaims()
	negativeSubject.Subject = "-5"

	sigStart := strings.LastIndex(valid, ".") + 1
	replacement := "A"
	if valid[sigStart] == 'A' {
		replacement = "B"
	}
	tampered := valid[:sigStart] + replacement + valid[sigStart+1:]

	tests := []struct {
		name    string
		token   string
		wantJWT error
	}{
		{name: "empty string", token: ""},
		{name: "garbage", token: "not.a.token"},
		{name: "tampered signature", token: tampered, wantJWT: jwt.ErrTokenSignatureInvalid},
		{name: "foreign secret", token: signToken(t, jwt.SigningMethodHS256, validClaims(), []byte(strings.Repeat("x", MinSecretLength))), wantJWT: jwt.ErrTokenSignatureInvalid},
		{name: "alg none", token: signToken(t, jwt.SigningMethodNone, validClaims(), jwt.UnsafeAllowNoneSignatureType), wantJWT: jwt.ErrTokenSignatureInvalid},
		{name: "alg HS512", token: signToken(t, jwt.SigningMethodHS512, validClaims(), testSecret), wantJWT: jwt.ErrTokenSignatureInvalid},
		{name: "expired", token: signToken(t, jwt.SigningMethodHS256, expired, testSecret), wantJWT: jwt.ErrTokenExpired},
		{name: "no exp", token: signToken(t, jwt.SigningMethodHS256, noExp, testSecret), wantJWT: jwt.ErrTokenRequiredClaimMissing},
		{name: "iat in future", token: signToken(t, jwt.SigningMethodHS256, futureIat, testSecret), wantJWT: jwt.ErrTokenUsedBeforeIssued},
		{name: "foreign issuer", token: signToken(t, jwt.SigningMethodHS256, foreignIssuer, testSecret), wantJWT: jwt.ErrTokenInvalidIssuer},
		{name: "no issuer", token: signToken(t, jwt.SigningMethodHS256, noIssuer, testSecret), wantJWT: jwt.ErrTokenRequiredClaimMissing},
		{name: "text subject", token: signToken(t, jwt.SigningMethodHS256, textSubject, testSecret)},
		{name: "empty subject", token: signToken(t, jwt.SigningMethodHS256, emptySubject, testSecret)},
		{name: "zero subject", token: signToken(t, jwt.SigningMethodHS256, zeroSubject, testSecret)},
		{name: "negative subject", token: signToken(t, jwt.SigningMethodHS256, negativeSubject, testSecret)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := m.Parse(tt.token)
			require.ErrorIs(t, err, ErrInvalidToken)
			if tt.wantJWT != nil {
				assert.ErrorIs(t, err, tt.wantJWT)
			}
			assert.Zero(t, userID)
		})
	}
}
