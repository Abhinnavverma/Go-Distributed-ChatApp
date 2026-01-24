package user

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo      *Repository
	jwtSecret string
}

type MyJWTClaims struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

func NewService(repo *Repository, secret string) *Service {
	return &Service{
		repo:      repo,
		jwtSecret: secret,
	}
}

func (s *Service) Register(ctx context.Context, req *RegisterRequest) (*RegisterRequest, error) {
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.MinCost)
	if err != nil {
		return nil, err
	}

	u := &User{
		Username: req.Username,
		Password: string(hashedPwd),
	}

	if _, err := s.repo.CreateUser(ctx, u); err != nil {
		return nil, err
	}

	return &RegisterRequest{Username: u.Username}, nil
}

func (s *Service) Login(ctx context.Context, req *RegisterRequest) (*LoginResponse, error) {
	u, err := s.repo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)); err != nil {
		return nil, err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, MyJWTClaims{
		ID:       u.ID,
		Username: u.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "go-chat-app",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	ss, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		AccessToken: ss,
		ID:          u.ID,
		Username:    u.Username,
	}, nil
}

func (s *Service) ValidateToken(tokenString string) (int, string, error) {
	claims := &MyJWTClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.jwtSecret), nil
	})

	if err != nil || !token.Valid {
		return 0, "", err
	}

	return claims.ID, claims.Username, nil
}

func (s *Service) SearchUsers(ctx context.Context, query string) ([]User, error) {
	return s.repo.SearchUsers(ctx, query)
}
