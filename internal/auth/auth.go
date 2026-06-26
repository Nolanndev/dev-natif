// Package auth issues and validates signed, expiring bearer tokens (JWT/HS256).
// It is transport-agnostic: the HTTP middleware lives in internal/http and calls
// Parse here. Tokens carry an expiry and can be renewed via Refresh.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidCredentials is returned by Login on a bad username/password.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidToken is returned by Parse/Refresh for a missing, malformed,
	// wrongly-signed or expired token.
	ErrInvalidToken = errors.New("invalid or expired token")
)

// Config configures the Authenticator.
type Config struct {
	Enabled  bool
	Username string
	Password string
	Secret   string        // HS256 signing secret; generated if empty
	TTL      time.Duration // token lifetime; defaults to 1h when <= 0
}

// Authenticator issues and validates bearer tokens for a single admin identity.
type Authenticator struct {
	enabled         bool
	user, pass      string
	secret          []byte
	ttl             time.Duration
	generatedSecret bool
}

// Token is a freshly issued credential.
type Token struct {
	Value     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	Subject   string    `json:"username"`
}

// New builds an Authenticator. If Secret is empty a random one is generated
// (tokens then do not survive a restart — set NATIF_JWT_SECRET in production).
func New(c Config) *Authenticator {
	secret := []byte(c.Secret)
	generated := false
	if len(secret) == 0 {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		secret = []byte(hex.EncodeToString(b))
		generated = true
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &Authenticator{
		enabled: c.Enabled, user: c.Username, pass: c.Password,
		secret: secret, ttl: ttl, generatedSecret: generated,
	}
}

// Enabled reports whether token auth is enforced.
func (a *Authenticator) Enabled() bool { return a.enabled }

// TTL is the configured token lifetime.
func (a *Authenticator) TTL() time.Duration { return a.ttl }

// UsesGeneratedSecret reports whether the signing secret was auto-generated.
func (a *Authenticator) UsesGeneratedSecret() bool { return a.generatedSecret }

// Login verifies credentials (constant-time) and issues a token.
func (a *Authenticator) Login(user, pass string) (Token, error) {
	okUser := subtle.ConstantTimeCompare([]byte(user), []byte(a.user)) == 1
	okPass := subtle.ConstantTimeCompare([]byte(pass), []byte(a.pass)) == 1
	if !okUser || !okPass {
		return Token{}, ErrInvalidCredentials
	}
	return a.issue(user)
}

// Refresh issues a new token from a still-valid one, extending the expiry.
func (a *Authenticator) Refresh(tokenStr string) (Token, error) {
	sub, _, err := a.Parse(tokenStr)
	if err != nil {
		return Token{}, err
	}
	return a.issue(sub)
}

// Parse validates a token's signature and expiry and returns its subject.
func (a *Authenticator) Parse(tokenStr string) (subject string, expiresAt time.Time, err error) {
	claims := &jwt.RegisteredClaims{}
	tok, perr := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return a.secret, nil
	})
	if perr != nil || !tok.Valid {
		return "", time.Time{}, ErrInvalidToken
	}
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return claims.Subject, expiresAt, nil
}

func (a *Authenticator) issue(subject string) (Token, error) {
	now := time.Now()
	exp := now.Add(a.ttl)
	claims := jwt.RegisteredClaims{
		Subject:   subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return Token{}, err
	}
	return Token{Value: signed, ExpiresAt: exp, Subject: subject}, nil
}
