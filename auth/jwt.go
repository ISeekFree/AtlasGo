package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"
)

const (
	// AlgorithmHS256 is the default HMAC algorithm.
	AlgorithmHS256 = "HS256"
	AlgorithmHS384 = "HS384"
	AlgorithmHS512 = "HS512"
)

var jwsHashers = map[string]func() hash.Hash{
	AlgorithmHS256: sha256.New,
	AlgorithmHS384: sha512.New384,
	AlgorithmHS512: sha512.New,
}

// JWTCodec signs and verifies HMAC JWTs. It only understands the standard
// alg/typ/exp/nbf fields; every business claim stays opaque, so each
// application defines its own claim schema inside a web.ContextLoader.
type JWTCodec struct {
	secret    []byte
	algorithm string
}

// NewJWTCodec returns an HS256 codec for the given shared secret.
func NewJWTCodec(secret []byte) *JWTCodec {
	return &JWTCodec{secret: secret, algorithm: AlgorithmHS256}
}

// NewJWTCodecWithAlgorithm returns a codec for one of the HS* algorithms.
func NewJWTCodecWithAlgorithm(secret []byte, algorithm string) (*JWTCodec, error) {
	normalized := strings.ToUpper(strings.TrimSpace(algorithm))
	if normalized == "" {
		normalized = AlgorithmHS256
	}
	if _, ok := jwsHashers[normalized]; !ok {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", algorithm)
	}
	return &JWTCodec{secret: secret, algorithm: normalized}, nil
}

// Encode signs the business claims. The caller owns every claim name.
func (c *JWTCodec) Encode(claims map[string]any) (string, error) {
	hasher, err := c.newHasher()
	if err != nil {
		return "", err
	}
	header, err := json.Marshal(map[string]string{"alg": c.algorithm, "typ": "JWT"})
	if err != nil {
		return "", err
	}
	if claims == nil {
		claims = map[string]any{}
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	signingInput := encodeSegment(header) + "." + encodeSegment(payload)
	hasher.Write([]byte(signingInput))
	return signingInput + "." + encodeSegment(hasher.Sum(nil)), nil
}

// Verify checks the signature and the registered time claims, returning the
// raw claim map.
func (c *JWTCodec) Verify(token string) (map[string]any, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return nil, errors.New("malformed token")
	}
	headerBytes, err := decodeSegment(parts[0])
	if err != nil {
		return nil, err
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, err
	}
	if header.Alg != c.algorithm {
		return nil, fmt.Errorf("unexpected JWT algorithm %q", header.Alg)
	}
	hasher, err := c.newHasher()
	if err != nil {
		return nil, err
	}
	hasher.Write([]byte(parts[0] + "." + parts[1]))
	expected := encodeSegment(hasher.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[2])) != 1 {
		return nil, errors.New("invalid token signature")
	}
	payloadBytes, err := decodeSegment(parts[1])
	if err != nil {
		return nil, err
	}
	claims := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, err
	}
	if err := validateTimeClaims(claims, time.Now()); err != nil {
		return nil, err
	}
	return claims, nil
}

// StripBearer removes an optional case-insensitive "Bearer " prefix.
func StripBearer(token string) string {
	trimmed := strings.TrimSpace(token)
	if len(trimmed) >= 7 && strings.EqualFold(trimmed[:7], "Bearer ") {
		return strings.TrimSpace(trimmed[7:])
	}
	return trimmed
}

func (c *JWTCodec) newHasher() (hash.Hash, error) {
	if len(c.secret) == 0 {
		return nil, errors.New("JWT secret is not configured")
	}
	factory, ok := jwsHashers[c.algorithm]
	if !ok {
		return nil, fmt.Errorf("unsupported JWT algorithm %q", c.algorithm)
	}
	return hmac.New(factory, c.secret), nil
}

func encodeSegment(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func decodeSegment(segment string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(segment)
}

func validateTimeClaims(claims map[string]any, now time.Time) error {
	if exp, ok := numericClaim(claims["exp"]); ok && !now.Before(time.Unix(exp, 0)) {
		return errors.New("token expired")
	}
	if nbf, ok := numericClaim(claims["nbf"]); ok && now.Before(time.Unix(nbf, 0)) {
		return errors.New("token not yet valid")
	}
	return nil
}

func numericClaim(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	default:
		return 0, false
	}
}
