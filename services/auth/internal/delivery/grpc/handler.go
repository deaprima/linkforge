package grpc

import (
	"context"

	"github.com/deaprima/linkforge/services/auth/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	authService service.AuthService
}

// membuat instance baru dari gRPC AuthHandler
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

func (h *AuthHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// panggil logika bisnis Register
	user, tokens, err := h.authService.Register(ctx, service.RegisterInput{
		Name:     req.GetName(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		if err.Error() == "email already exists" {
			return nil, status.Error(codes.AlreadyExists, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to register user: %v", err)
	}
	// bungkus ke format Protobuf RegisterResponse
	return &pb.RegisterResponse{
		User: &pb.User{
			Id:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
		},
		Tokens: &pb.TokenPair{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		},
	}, nil
}

func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// panggil logika bisnis Login
	user, tokens, err := h.authService.Login(ctx, service.LoginInput{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		if err.Error() == "invalid credentials" {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to login: %v", err)
	}
	// bungkus ke format Protobuf LoginResponse
	return &pb.LoginResponse{
		User: &pb.User{
			Id:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
		},
		Tokens: &pb.TokenPair{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		},
	}, nil
}