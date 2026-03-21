package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

// makeJWT creates a JWT token string for testing.
func makeJWT(header, claims map[string]any, secret string) string {
	hdr, _ := json.Marshal(header)
	clm, _ := json.Marshal(claims)

	h := base64.RawURLEncoding.EncodeToString(hdr)
	c := base64.RawURLEncoding.EncodeToString(clm)

	signingInput := h + "." + c
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return h + "." + c + "." + sig
}

func TestVerifyCallbackJWT(t *testing.T) {
	const botID = "bot-123"
	const secret = "my-secret-key"

	lookup := func(id string) (string, error) {
		if id == botID {
			return secret, nil
		}
		return "", fmt.Errorf("unknown bot: %s", id)
	}

	t.Run("valid token", func(t *testing.T) {
		now := time.Now().Unix()
		token := makeJWT(
			map[string]any{"alg": "HS256", "typ": "JWT"},
			map[string]any{"aud": botID, "exp": now + 3600, "nbf": now - 60},
			secret,
		)

		claims, err := verifyCallbackJWT(token, lookup)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if claims.Aud != botID {
			t.Fatalf("expected aud=%s, got %s", botID, claims.Aud)
		}
	})

	t.Run("valid token without exp/nbf", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"aud": botID},
			secret,
		)

		claims, err := verifyCallbackJWT(token, lookup)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if claims.Aud != botID {
			t.Fatalf("expected aud=%s, got %s", botID, claims.Aud)
		}
	})

	t.Run("malformed token", func(t *testing.T) {
		_, err := verifyCallbackJWT("not-a-jwt", lookup)
		if !errors.Is(err, errJWTMalformed) {
			t.Fatalf("expected errJWTMalformed, got: %v", err)
		}
	})

	t.Run("too many parts", func(t *testing.T) {
		_, err := verifyCallbackJWT("a.b.c.d", lookup)
		if !errors.Is(err, errJWTMalformed) {
			t.Fatalf("expected errJWTMalformed, got: %v", err)
		}
	})

	t.Run("algorithm none rejected", func(t *testing.T) {
		hdr, _ := json.Marshal(map[string]any{"alg": "none"})
		clm, _ := json.Marshal(map[string]any{"aud": botID})
		token := base64.RawURLEncoding.EncodeToString(hdr) + "." +
			base64.RawURLEncoding.EncodeToString(clm) + "."

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTAlgNone) {
			t.Fatalf("expected errJWTAlgNone, got: %v", err)
		}
	})

	t.Run("unsupported algorithm", func(t *testing.T) {
		hdr, _ := json.Marshal(map[string]any{"alg": "RS256"})
		clm, _ := json.Marshal(map[string]any{"aud": botID})
		token := base64.RawURLEncoding.EncodeToString(hdr) + "." +
			base64.RawURLEncoding.EncodeToString(clm) + ".fakesig"

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTAlgorithm) {
			t.Fatalf("expected errJWTAlgorithm, got: %v", err)
		}
	})

	t.Run("wrong signature", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"aud": botID},
			"wrong-secret",
		)

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTSignature) {
			t.Fatalf("expected errJWTSignature, got: %v", err)
		}
	})

	t.Run("missing aud claim", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"sub": "test"},
			secret,
		)

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTMissingAud) {
			t.Fatalf("expected errJWTMissingAud, got: %v", err)
		}
	})

	t.Run("unknown bot ID", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"aud": "unknown-bot"},
			secret,
		)

		_, err := verifyCallbackJWT(token, lookup)
		if err == nil {
			t.Fatal("expected error for unknown bot")
		}
		if !contains(err.Error(), "unknown bot") {
			t.Fatalf("expected 'unknown bot' in error, got: %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"aud": botID, "exp": time.Now().Unix() - 3600},
			secret,
		)

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTExpired) {
			t.Fatalf("expected errJWTExpired, got: %v", err)
		}
	})

	t.Run("not yet valid", func(t *testing.T) {
		token := makeJWT(
			map[string]any{"alg": "HS256"},
			map[string]any{"aud": botID, "nbf": time.Now().Unix() + 3600},
			secret,
		)

		_, err := verifyCallbackJWT(token, lookup)
		if !errors.Is(err, errJWTNotYetValid) {
			t.Fatalf("expected errJWTNotYetValid, got: %v", err)
		}
	})

	t.Run("invalid header encoding", func(t *testing.T) {
		_, err := verifyCallbackJWT("!!!.YQ.YQ", lookup)
		if err == nil || !contains(err.Error(), "header encoding") {
			t.Fatalf("expected header encoding error, got: %v", err)
		}
	})

	t.Run("invalid claims encoding", func(t *testing.T) {
		hdr, _ := json.Marshal(map[string]any{"alg": "HS256"})
		token := base64.RawURLEncoding.EncodeToString(hdr) + ".!!!.YQ"

		_, err := verifyCallbackJWT(token, lookup)
		if err == nil || !contains(err.Error(), "claims encoding") {
			t.Fatalf("expected claims encoding error, got: %v", err)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
