package application

import (
	"backend/internal/modules/auth/domain"
	users "backend/internal/modules/users/application"
	userdomain "backend/internal/modules/users/domain"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type Service struct {
	users      users.UserRepository
	tokens     RefreshTokenRepository
	jwt        TokenService
	refreshTTL time.Duration
}

func NewService(u users.UserRepository, t RefreshTokenRepository, j TokenService, ttl time.Duration) *Service {
	return &Service{users: u, tokens: t, jwt: j, refreshTTL: ttl}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (s *Service) LoginPassword(ctx context.Context, username, password, agent, ip string) (*TokenPair, error) {
	u, passwordHash, err := s.users.FindPasswordCredential(ctx, username)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if u.Role == userdomain.RoleWorker || !u.Active {
		return nil, fmt.Errorf("invalid credentials")
	}
	return s.issue(ctx, u, agent, ip)
}
func (s *Service) LoginDNI(ctx context.Context, dni, agent, ip string) (*TokenPair, error) {
	u, err := s.users.FindByDNI(ctx, dni)
	if err != nil || u.Role != userdomain.RoleWorker || !u.Active {
		return nil, fmt.Errorf("invalid credentials")
	}
	return s.issue(ctx, u, agent, ip)
}
func (s *Service) issue(ctx context.Context, u *userdomain.User, agent, ip string) (*TokenPair, error) {
	access, err := s.jwt.CreateAccessToken(u.ID, string(u.Role))
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return nil, err
	}
	refresh := base64.RawURLEncoding.EncodeToString(raw)
	t := &domain.RefreshToken{UserID: u.ID, TokenHash: s.jwt.HashRefreshToken(refresh), ExpiresAt: time.Now().Add(s.refreshTTL), UserAgent: agent, IPAddress: ip}
	if err = s.tokens.Create(ctx, t); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: 300}, nil
}
func (s *Service) Refresh(ctx context.Context, raw, agent, ip string) (*TokenPair, error) {
	old, err := s.tokens.FindValidByHash(ctx, s.jwt.HashRefreshToken(raw), time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	u, err := s.users.FindByID(ctx, old.UserID)
	if err != nil || !u.Active {
		return nil, fmt.Errorf("invalid refresh token")
	}
	if err = s.tokens.Revoke(ctx, old.ID, time.Now()); err != nil {
		return nil, err
	}
	return s.issue(ctx, u, agent, ip)
}
