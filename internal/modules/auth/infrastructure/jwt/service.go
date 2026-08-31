package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Service struct {
	secret []byte
}

func New(secret string) *Service {
	return &Service{secret: []byte(secret)}
}
func (s *Service) CreateAccessToken(userID, role string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{"sub": userID, "role": role, "iat": now.Unix(), "exp": now.Add(ttl).Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}
func (s *Service) HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *Service) Parse(token string) (jwt.MapClaims, error) {
	parsed, err := jwt.Parse(token, func(t *jwt.Token) (any, error) { return s.secret, nil }, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return parsed.Claims.(jwt.MapClaims), nil
}
