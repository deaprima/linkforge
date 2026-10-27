package grpc

import (
	"context"
	"errors"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/deaprima/linkforge/services/analytics/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AnalyticsHandler struct {
	pb.UnimplementedAnalyticsServiceServer
	service service.AnalyticsService
	hub     *realtime.Hub
}

func NewAnalyticsHandler(svc service.AnalyticsService, hub *realtime.Hub) *AnalyticsHandler {
	return &AnalyticsHandler{service: svc, hub: hub}
}

func mapError(err error) error {
	if errors.Is(err, service.ErrInvalidArgument) {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return status.Errorf(codes.Internal, "internal error: %v", err)
}

func toCounts[T any](items []repository.CountItem, build func(key string, count int64) T) []T {
	out := make([]T, len(items))
	for i, it := range items {
		out[i] = build(it.Key, it.Count)
	}
	return out
}

// GetLinkAnalytics — ownership link sudah divalidasi Gateway via Link Service sebelum memanggil ini.
func (h *AnalyticsHandler) GetLinkAnalytics(ctx context.Context, req *pb.GetLinkAnalyticsRequest) (*pb.GetLinkAnalyticsResponse, error) {
	res, err := h.service.GetLinkAnalytics(ctx, req.GetLinkId(), req.GetFrom(), req.GetTo())
	if err != nil {
		return nil, mapError(err)
	}

	series := make([]*pb.DailyClickCount, len(res.TimeSeries))
	for i, d := range res.TimeSeries {
		series[i] = &pb.DailyClickCount{Date: d.Date.Format("2006-01-02"), Clicks: d.Clicks}
	}

	return &pb.GetLinkAnalyticsResponse{
		LinkId:       res.LinkID.String(),
		TotalClicks:  res.TotalClicks,
		UniqueClicks: res.UniqueClicks,
		TopCountries: toCounts(res.TopCountries, func(k string, n int64) *pb.CountryCount { return &pb.CountryCount{Country: k, Count: n} }),
		TopBrowsers:  toCounts(res.TopBrowsers, func(k string, n int64) *pb.BrowserCount { return &pb.BrowserCount{Browser: k, Count: n} }),
		TopDevices:   toCounts(res.TopDevices, func(k string, n int64) *pb.DeviceCount { return &pb.DeviceCount{Device: k, Count: n} }),
		TimeSeries:   series,
	}, nil
}

func (h *AnalyticsHandler) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.GetSummaryResponse, error) {
	res, err := h.service.GetSummary(ctx, req.GetLinkIds())
	if err != nil {
		return nil, mapError(err)
	}

	top := make([]*pb.TopLink, len(res.TopLinks))
	for i, l := range res.TopLinks {
		top[i] = &pb.TopLink{LinkId: l.LinkID.String(), Alias: l.Alias, Clicks: l.Clicks}
	}
	return &pb.GetSummaryResponse{
		TotalLinks:  int64(res.TotalLinks),
		TotalClicks: res.TotalClicks,
		TopLinks:    top,
	}, nil
}

// SubscribeRealtime men-stream klik baru milik user sampai client disconnect.
func (h *AnalyticsHandler) SubscribeRealtime(req *pb.SubscribeRealtimeRequest, stream pb.AnalyticsService_SubscribeRealtimeServer) error {
	if req.GetUserId() == "" {
		return status.Error(codes.InvalidArgument, "user_id is required")
	}

	sub := h.hub.Subscribe(req.GetUserId(), req.GetLinkIds())
	defer h.hub.Unsubscribe(sub)

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case click, ok := <-sub.C:
			if !ok {
				return nil
			}
			err := stream.Send(&pb.ClickEvent{
				LinkId:    click.LinkID,
				Alias:     click.Alias,
				Country:   click.Country,
				Browser:   click.Browser,
				Device:    click.Device,
				Timestamp: click.ClickedAt.UTC().Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
		}
	}
}
