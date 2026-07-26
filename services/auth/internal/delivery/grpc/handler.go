package grpc

import (
	"context"
	"strings"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (h *AuthHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	claims, err := h.authService.ValidateToken(ctx, req.GetAccessToken())
	if err != nil {
		return &pb.ValidateTokenResponse{
			IsValid: false,
		}, nil
	}

	return &pb.ValidateTokenResponse{
		IsValid: true,
		UserId:  claims.UserID,
		Email:   claims.Email,
	}, nil
}

func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	// Ambil IP dan User Agent dari gRPC metadata (jika dikirim oleh API Gateway)
	ip := ""
	userAgent := ""
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if ips := md.Get("x-forwarded-for"); len(ips) > 0 {
			ip = ips[0]
		}
		if uas := md.Get("user-agent"); len(uas) > 0 {
			userAgent = uas[0]
		}
	}

	tokens, err := h.authService.RefreshToken(ctx, req.GetRefreshToken(), ip, userAgent)
	if err != nil {
		if err.Error() == "refresh token abuse detected; all sessions revoked" {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &pb.RefreshTokenResponse{
		Tokens: &pb.TokenPair{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
		},
	}, nil
}

func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	err := h.authService.Logout(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to logout: %v", err)
	}

	return &pb.LogoutResponse{
		Success: true,
	}, nil
}

func (h *AuthHandler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	user, err := h.authService.GetUser(ctx, userID)
	if err != nil {
		if err.Error() == "user not found" {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}

	return &pb.GetUserResponse{
		User: &pb.User{
			Id:		user.ID.String(),
			Name:	user.Name,
			Email: 	user.Email,
		},
	}, nil
}

// Helper: konversi entity.ApiKey → pb.ApiKey
func toProtoApiKey(k *entity.ApiKey) *pb.ApiKey {
    proto := &pb.ApiKey{
        Id:        k.ID.String(),
        Name:      k.Name,
        CreatedAt: timestamppb.New(k.CreatedAt),
    }
    if k.LastUsedAt != nil {
        proto.LastUsedAt = timestamppb.New(*k.LastUsedAt)
    }
    return proto
}
func (h *AuthHandler) CreateApiKey(ctx context.Context, req *pb.CreateApiKeyRequest) (*pb.CreateApiKeyResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }
    key, rawKey, err := h.authService.CreateApiKey(ctx, userID, req.GetName())
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to create api key: %v", err)
    }
    return &pb.CreateApiKeyResponse{
        ApiKey: toProtoApiKey(key),
        RawKey: rawKey, // dikirim SEKALI ke client
    }, nil
}
func (h *AuthHandler) ListApiKeys(ctx context.Context, req *pb.ListApiKeysRequest) (*pb.ListApiKeysResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }
    keys, err := h.authService.ListApiKeys(ctx, userID)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "failed to list api keys: %v", err)
    }
    var protoKeys []*pb.ApiKey
    for i := range keys {
        protoKeys = append(protoKeys, toProtoApiKey(&keys[i]))
    }
    return &pb.ListApiKeysResponse{ApiKeys: protoKeys}, nil
}
func (h *AuthHandler) DeleteApiKey(ctx context.Context, req *pb.DeleteApiKeyRequest) (*pb.DeleteApiKeyResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }
    keyID, err := uuid.Parse(req.GetApiKeyId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid api_key_id format")
    }
    if err := h.authService.DeleteApiKey(ctx, userID, keyID); err != nil {
        return nil, status.Errorf(codes.Internal, "failed to delete api key: %v", err)
    }
    return &pb.DeleteApiKeyResponse{Success: true}, nil
}
func (h *AuthHandler) ValidateApiKey(ctx context.Context, req *pb.ValidateApiKeyRequest) (*pb.ValidateApiKeyResponse, error) {
    key, err := h.authService.ValidateApiKey(ctx, req.GetApiKey())
    if err != nil {
        // Jika invalid/revoked, kembalikan is_valid=false, bukan error gRPC
        return &pb.ValidateApiKeyResponse{IsValid: false}, nil
    }
    return &pb.ValidateApiKeyResponse{
        IsValid: true,
        UserId:  key.UserID.String(),
    }, nil
}

func (h *AuthHandler) GoogleAuth(ctx context.Context, req *pb.GoogleAuthRequest) (*pb.GoogleAuthResponse, error) {
    user, tokens, err := h.authService.GoogleAuth(ctx, req.GetIdToken())
    if err != nil {
        if strings.Contains(err.Error(), "invalid google id token") {
            return nil, status.Error(codes.Unauthenticated, err.Error())
        }
        return nil, status.Errorf(codes.Internal, "google auth failed: %v", err)
    }

    return &pb.GoogleAuthResponse{
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
