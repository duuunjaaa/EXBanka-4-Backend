package handlers

import (
	"context"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb_auth "github.com/exbanka/backend/shared/pb/auth"
	pb_emp "github.com/exbanka/backend/shared/pb/employee"
)

const jwtSecret = "secret-key-change-in-production"

type AuthServer struct {
	pb_auth.UnimplementedAuthServiceServer
	EmployeeClient pb_emp.EmployeeServiceClient
}

func (s *AuthServer) Login(ctx context.Context, req *pb_auth.LoginRequest) (*pb_auth.LoginResponse, error) {
	creds, err := s.EmployeeClient.GetEmployeeCredentials(ctx, &pb_emp.GetEmployeeCredentialsRequest{
		Username: req.Username,
	})
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if creds.PasswordHash == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if !creds.Aktivan {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(creds.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	accessToken, err := generateToken(creds.Id, req.Username, "access", creds.Dozvole, 15*time.Minute)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	refreshToken, err := generateToken(creds.Id, req.Username, "refresh", creds.Dozvole, 7*24*time.Hour)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb_auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthServer) Refresh(_ context.Context, req *pb_auth.RefreshRequest) (*pb_auth.RefreshResponse, error) {
	token, err := jwt.Parse(req.RefreshToken, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid token claims")
	}

	if claims["type"] != "refresh" {
		return nil, status.Error(codes.Unauthenticated, "invalid token type")
	}

	userID := int64(claims["user_id"].(float64))
	username := claims["username"].(string)

	var dozvole []string
	if raw, ok := claims["dozvole"].([]interface{}); ok {
		for _, d := range raw {
			if s, ok := d.(string); ok {
				dozvole = append(dozvole, s)
			}
		}
	}

	accessToken, err := generateToken(userID, username, "access", dozvole, 15*time.Minute)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb_auth.RefreshResponse{AccessToken: accessToken}, nil
}

func generateToken(userID int64, username, tokenType string, dozvole []string, d time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"type":     tokenType,
		"dozvole":  dozvole,
		"exp":      time.Now().Add(d).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(jwtSecret))
}
