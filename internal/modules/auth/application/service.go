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
	accessTTL  time.Duration
	workerTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(u users.UserRepository, t RefreshTokenRepository, j TokenService, accessTTL, workerTTL, refreshTTL time.Duration) *Service {
	return &Service{users: u, tokens: t, jwt: j, accessTTL: accessTTL, workerTTL: workerTTL, refreshTTL: refreshTTL}
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
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
	return s.issueWithRefresh(ctx, u, agent, ip)
}
func (s *Service) LoginDNI(ctx context.Context, dni, password, agent, ip string) (*TokenPair, error) {
	u, passwordHash, err := s.users.FindWorkerPasswordCredential(ctx, dni)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil || u.Role != userdomain.RoleWorker || !u.Active {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err = s.tokens.RevokeAllByUser(ctx, u.ID, time.Now()); err != nil {
		return nil, err
	}
	return s.issueWorker(u)
}
func (s *Service) issueWorker(u *userdomain.User) (*TokenPair, error) {
	access, err := s.jwt.CreateAccessToken(u.ID, string(u.Role), s.workerTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(s.workerTTL / time.Second)}, nil
}
func (s *Service) issueWithRefresh(ctx context.Context, u *userdomain.User, agent, ip string) (*TokenPair, error) {
	access, err := s.jwt.CreateAccessToken(u.ID, string(u.Role), s.accessTTL)
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
	return &TokenPair{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.accessTTL / time.Second)}, nil
}
func (s *Service) Refresh(ctx context.Context, raw, agent, ip string) (*TokenPair, error) {
	now := time.Now()
	old, err := s.tokens.FindValidByHash(ctx, s.jwt.HashRefreshToken(raw), now)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	u, err := s.users.FindByID(ctx, old.UserID)
	if err != nil || !u.Active || u.Role == userdomain.RoleWorker {
		if err == nil && u.Role == userdomain.RoleWorker {
			_ = s.tokens.RevokeAllByUser(ctx, u.ID, time.Now())
		}
		return nil, fmt.Errorf("invalid refresh token")
	}
	access, err := s.jwt.CreateAccessToken(u.ID, string(u.Role), s.accessTTL)
	if err != nil {
		return nil, err
	}
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return nil, err
	}
	refresh := base64.RawURLEncoding.EncodeToString(random)
	replacement := &domain.RefreshToken{UserID: u.ID, TokenHash: s.jwt.HashRefreshToken(refresh), ExpiresAt: now.Add(s.refreshTTL), UserAgent: agent, IPAddress: ip}
	if err = s.tokens.Rotate(ctx, old.ID, replacement, now); err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: int64(s.accessTTL / time.Second)}, nil
}
