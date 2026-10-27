package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/deaprima/linkforge/services/analytics/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	"github.com/google/uuid"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type fakeService struct {
	link    *service.LinkAnalytics
	summary *service.Summary
	err     error
}

func (f *fakeService) GetLinkAnalytics(context.Context, string, string, string) (*service.LinkAnalytics, error) {
	return f.link, f.err
}
func (f *fakeService) GetSummary(context.Context, []string) (*service.Summary, error) {
	return f.summary, f.err
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %s, want %s (err %v)", got, want, err)
	}
}

func TestGetLinkAnalytics(t *testing.T) {
	linkID := uuid.New()
	day := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	h := NewAnalyticsHandler(&fakeService{link: &service.LinkAnalytics{
		LinkID: linkID, TotalClicks: 254, UniqueClicks: 180,
		TopCountries: []repository.CountItem{{Key: "ID", Count: 120}},
		TopBrowsers:  []repository.CountItem{{Key: "Chrome", Count: 150}},
		TopDevices:   []repository.CountItem{{Key: "mobile", Count: 180}},
		TimeSeries:   []repository.DailyCount{{Date: day, Clicks: 30}},
	}}, realtime.NewHub())

	res, err := h.GetLinkAnalytics(context.Background(), &pb.GetLinkAnalyticsRequest{LinkId: linkID.String()})
	if err != nil {
		t.Fatal(err)
	}
	if res.LinkId != linkID.String() || res.TotalClicks != 254 || res.UniqueClicks != 180 ||
		res.TopCountries[0].Country != "ID" || res.TopBrowsers[0].Browser != "Chrome" || res.TopDevices[0].Device != "mobile" ||
		res.TimeSeries[0].Date != "2026-07-01" || res.TimeSeries[0].Clicks != 30 {
		t.Errorf("response = %v", res)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		err  error
		want codes.Code
	}{
		{fmt.Errorf("%w: from must not be after to", service.ErrInvalidArgument), codes.InvalidArgument},
		{errors.New("failed to count clicks: db down"), codes.Internal},
	}
	for _, tt := range tests {
		h := NewAnalyticsHandler(&fakeService{err: tt.err}, realtime.NewHub())
		_, err := h.GetLinkAnalytics(context.Background(), &pb.GetLinkAnalyticsRequest{})
		wantCode(t, err, tt.want)
		_, err = h.GetSummary(context.Background(), &pb.GetSummaryRequest{})
		wantCode(t, err, tt.want)
	}

	_, err := NewAnalyticsHandler(&fakeService{err: tests[0].err}, realtime.NewHub()).GetLinkAnalytics(context.Background(), &pb.GetLinkAnalyticsRequest{})
	if msg := status.Convert(err).Message(); msg != "from must not be after to" {
		t.Errorf("client message = %q, internal sentinel prefix must be stripped", msg)
	}
}

func TestGetSummary(t *testing.T) {
	linkID := uuid.New()
	h := NewAnalyticsHandler(&fakeService{summary: &service.Summary{
		TotalLinks: 10, TotalClicks: 1280,
		TopLinks: []repository.LinkClicks{{LinkID: linkID, Alias: "my-alias", Clicks: 254}},
	}}, realtime.NewHub())

	res, err := h.GetSummary(context.Background(), &pb.GetSummaryRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalLinks != 10 || res.TotalClicks != 1280 || res.TopLinks[0].Alias != "my-alias" || res.TopLinks[0].LinkId != linkID.String() {
		t.Errorf("response = %v", res)
	}
}

// startServer menjalankan handler di gRPC server sungguhan supaya streaming teruji end-to-end.
func startServer(t *testing.T, hub *realtime.Hub) pb.AnalyticsServiceClient {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpclib.NewServer()
	pb.RegisterAnalyticsServiceServer(srv, NewAnalyticsHandler(&fakeService{}, hub))
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)

	conn, err := grpclib.NewClient(lis.Addr().String(), grpclib.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return pb.NewAnalyticsServiceClient(conn)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestSubscribeRealtime_StreamsOwnClicks(t *testing.T) {
	hub := realtime.NewHub()
	client := startServer(t, hub)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.SubscribeRealtime(ctx, &pb.SubscribeRealtimeRequest{UserId: "user-1"})
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return hub.Subscribers() == 1 })

	at := time.Date(2026, 7, 8, 7, 15, 0, 0, time.UTC)
	hub.Publish(realtime.Click{UserID: "user-2", LinkID: "other"}) // milik user lain, tidak boleh terkirim
	hub.Publish(realtime.Click{UserID: "user-1", LinkID: "link-1", Alias: "promo", Country: "ID", Browser: "Chrome", Device: "mobile", ClickedAt: at})

	ev, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if ev.LinkId != "link-1" || ev.Alias != "promo" || ev.Country != "ID" || ev.Timestamp != "2026-07-08T07:15:00Z" {
		t.Errorf("event = %v", ev)
	}

	// Client disconnect → subscription dibersihkan dari hub
	cancel()
	waitFor(t, func() bool { return hub.Subscribers() == 0 })
}

func TestSubscribeRealtime_RequiresUserID(t *testing.T) {
	client := startServer(t, realtime.NewHub())
	stream, err := client.SubscribeRealtime(context.Background(), &pb.SubscribeRealtimeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = stream.Recv()
	wantCode(t, err, codes.InvalidArgument)
}
