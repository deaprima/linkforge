package grpc

import (
    "context"
    "strings"
    "time"

    "github.com/deaprima/linkforge/services/link/internal/service"
    pb "github.com/deaprima/linkforge/services/shared/proto/link"
    "github.com/google/uuid"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/timestamppb"
)

type LinkHandler struct {
    pb.UnimplementedLinkServiceServer
    linkService service.LinkService
}

func NewLinkHandler(linkService service.LinkService) *LinkHandler {
    return &LinkHandler{linkService: linkService}
}

// toProtoLink mengkonversi *service.LinkView → *pb.Link.
func toProtoLink(l *service.LinkView) *pb.Link {
    proto := &pb.Link{
        Id:          l.ID.String(),
        UserId:      l.UserID.String(),
        OriginalUrl: l.OriginalURL,
        Alias:       l.Alias,
        ClickCount:  l.ClickCount,
        HasPassword: l.HasPassword,
        CreatedAt:   timestamppb.New(l.CreatedAt),
        UpdatedAt:   timestamppb.New(l.UpdatedAt),
    }
    if l.ExpiredAt != nil {
        proto.ExpiredAt = timestamppb.New(*l.ExpiredAt)
    }
    return proto
}

// mapError mengkonversi service error ke gRPC status error.
func mapError(err error) error {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "not found"):
        return status.Error(codes.NotFound, msg)
    case strings.Contains(msg, "is blocked"),
        strings.Contains(msg, "invalid url"),
        strings.Contains(msg, "cannot be empty"),
        strings.Contains(msg, "alias") && strings.Contains(msg, "reserved"),
        strings.Contains(msg, "alias hanya boleh"):
        return status.Error(codes.InvalidArgument, msg)
    case strings.Contains(msg, "duplicate key"),
        strings.Contains(msg, "unique constraint"),
        strings.Contains(msg, "already taken"):
        return status.Error(codes.AlreadyExists, "alias is already taken")
    default:
        return status.Errorf(codes.Internal, "internal error: %v", err)
    }
}

func (h *LinkHandler) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    var expiredAt *time.Time
    if req.GetExpiredAt() != nil {
        t := req.GetExpiredAt().AsTime()
        expiredAt = &t
    }

    link, err := h.linkService.CreateLink(ctx, service.CreateLinkInput{
        UserID:      userID,
        OriginalURL: req.GetOriginalUrl(),
        Alias:       req.GetAlias(),
        ExpiredAt:   expiredAt,
        Password:    req.GetPassword(),
    })
    if err != nil {
        return nil, mapError(err)
    }

    return &pb.CreateLinkResponse{Link: toProtoLink(link)}, nil
}

func (h *LinkHandler) GetLink(ctx context.Context, req *pb.GetLinkRequest) (*pb.GetLinkResponse, error) {
    linkID, err := uuid.Parse(req.GetLinkId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid link_id format")
    }
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    link, err := h.linkService.GetLink(ctx, linkID, userID)
    if err != nil {
        return nil, mapError(err)
    }

    return &pb.GetLinkResponse{Link: toProtoLink(link)}, nil
}

func (h *LinkHandler) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    links, total, err := h.linkService.ListLinks(ctx, userID, int(req.GetPage()), int(req.GetLimit()))
    if err != nil {
        return nil, mapError(err)
    }

    protoLinks := make([]*pb.Link, len(links))
    for i, l := range links {
        protoLinks[i] = toProtoLink(l)
    }

    return &pb.ListLinksResponse{
        Links: protoLinks,
        Total: int32(total),
    }, nil
}

func (h *LinkHandler) UpdateLink(ctx context.Context, req *pb.UpdateLinkRequest) (*pb.UpdateLinkResponse, error) {
    linkID, err := uuid.Parse(req.GetLinkId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid link_id format")
    }
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    var expiredAt *time.Time
    clearExpiry := false
    if req.GetExpiredAt() != nil {
        t := req.GetExpiredAt().AsTime()
        expiredAt = &t
    } else {
        clearExpiry = true
    }

    link, err := h.linkService.UpdateLink(ctx, service.UpdateLinkInput{
        ID:          linkID,
        UserID:      userID,
        OriginalURL: req.GetOriginalUrl(),
        Alias:       req.GetAlias(),
        ExpiredAt:   expiredAt,
        ClearExpiry: clearExpiry,
    })
    if err != nil {
        return nil, mapError(err)
    }

    return &pb.UpdateLinkResponse{Link: toProtoLink(link)}, nil
}

func (h *LinkHandler) DeleteLink(ctx context.Context, req *pb.DeleteLinkRequest) (*pb.DeleteLinkResponse, error) {
    linkID, err := uuid.Parse(req.GetLinkId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid link_id format")
    }
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    if err := h.linkService.DeleteLink(ctx, linkID, userID); err != nil {
        return nil, mapError(err)
    }

    return &pb.DeleteLinkResponse{Success: true}, nil
}

func (h *LinkHandler) ResolveAlias(ctx context.Context, req *pb.ResolveAliasRequest) (*pb.ResolveAliasResponse, error) {
    result, err := h.linkService.ResolveAlias(ctx, req.GetAlias())
    if err != nil {
        return nil, mapError(err)
    }

    return &pb.ResolveAliasResponse{
        LinkId:      result.LinkID.String(),
        OriginalUrl: result.OriginalURL,
        IsExpired:   result.IsExpired,
        HasPassword: result.HasPassword,
    }, nil
}

func (h *LinkHandler) VerifyLinkPassword(ctx context.Context, req *pb.VerifyLinkPasswordRequest) (*pb.VerifyLinkPasswordResponse, error) {
    link, err := h.linkService.VerifyLinkPassword(ctx, req.GetAlias(), req.GetPassword())
    if err != nil {
        if err.Error() == "invalid password" {
            return nil, status.Error(codes.Unauthenticated, err.Error())
        }
        return nil, mapError(err)
    }

    return &pb.VerifyLinkPasswordResponse{
        IsValid:     true,
        LinkId:      link.ID.String(),
        OriginalUrl: link.OriginalURL,
    }, nil
}

func (h *LinkHandler) GetUserLinkIDs(ctx context.Context, req *pb.GetUserLinkIDsRequest) (*pb.GetUserLinkIDsResponse, error) {
    userID, err := uuid.Parse(req.GetUserId())
    if err != nil {
        return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
    }

    ids, err := h.linkService.GetUserLinkIDs(ctx, userID)
    if err != nil {
        return nil, mapError(err)
    }

    strIDs := make([]string, len(ids))
    for i, id := range ids {
        strIDs[i] = id.String()
    }

    return &pb.GetUserLinkIDsResponse{LinkIds: strIDs}, nil
}
