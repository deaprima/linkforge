package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/link/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeLinkService mengembalikan nilai yang dikonfigurasi dan mencatat input terakhir.
type fakeLinkService struct {
	link    *service.LinkView
	links   []*service.LinkView
	total   int64
	resolve *service.ResolveResult
	ids     []uuid.UUID
	err     error

	gotCreate      service.CreateLinkInput
	gotUpdate      service.UpdateLinkInput
	gotPage        int
	gotLimit       int
	gotID, gotUser uuid.UUID
	gotClient      service.ClientInfo
}

func (f *fakeLinkService) CreateLink(_ context.Context, in service.CreateLinkInput) (*service.LinkView, error) {
	f.gotCreate = in
	return f.link, f.err
}
func (f *fakeLinkService) GetLink(_ context.Context, id, userID uuid.UUID) (*service.LinkView, error) {
	f.gotID, f.gotUser = id, userID
	return f.link, f.err
}
func (f *fakeLinkService) ListLinks(_ context.Context, _ uuid.UUID, page, limit int) ([]*service.LinkView, int64, error) {
	f.gotPage, f.gotLimit = page, limit
	return f.links, f.total, f.err
}
func (f *fakeLinkService) UpdateLink(_ context.Context, in service.UpdateLinkInput) (*service.LinkView, error) {
	f.gotUpdate = in
	return f.link, f.err
}
func (f *fakeLinkService) DeleteLink(_ context.Context, id, userID uuid.UUID) error {
	f.gotID, f.gotUser = id, userID
	return f.err
}
func (f *fakeLinkService) ResolveAlias(_ context.Context, _ string, client service.ClientInfo) (*service.ResolveResult, error) {
	f.gotClient = client
	return f.resolve, f.err
}
func (f *fakeLinkService) VerifyLinkPassword(_ context.Context, _, _ string, client service.ClientInfo) (*service.LinkView, error) {
	f.gotClient = client
	return f.link, f.err
}
func (f *fakeLinkService) GetUserLinkIDs(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return f.ids, f.err
}

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %s, want %s (err %v)", got, want, err)
	}
}

func testView() *service.LinkView {
	now := time.Date(2026, 7, 8, 7, 0, 0, 0, time.UTC)
	return &service.LinkView{ID: uuid.New(), UserID: uuid.New(), OriginalURL: "https://a.com", Alias: "abc123", ClickCount: 5, CreatedAt: now, UpdatedAt: now}
}

// Pesan-pesan ini harus sama persis dengan yang dihasilkan service layer;
// kalau pesan di service berubah, test ini yang menangkapnya.
func TestMapError(t *testing.T) {
	tests := []struct {
		msg  string
		want codes.Code
	}{
		{"link not found", codes.NotFound},
		{"failed to get link: link not found", codes.NotFound},
		{"failed to delete link: link not found or not owned by user", codes.NotFound},
		{"link has expired", codes.FailedPrecondition},
		{"link is not password protected", codes.InvalidArgument},
		{"url domain is blocked", codes.InvalidArgument},
		{"invalid url: invalid URL: nope", codes.InvalidArgument},
		{"original_url cannot be empty", codes.InvalidArgument},
		{"alias 'api' is reserved", codes.InvalidArgument},
		{"alias hanya boleh huruf, angka, tanda hubung (-) dan underscore (_), panjang 3-64 karakter", codes.InvalidArgument},
		{`failed to create link: ERROR: duplicate key value violates unique constraint "idx_links_custom_alias"`, codes.AlreadyExists},
		{"failed to update link: unique constraint failed", codes.AlreadyExists},
		{"failed to generate unique short code after retries", codes.Internal},
		{"failed to list links: connection refused", codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			wantCode(t, mapError(errors.New(tt.msg)), tt.want)
		})
	}
}

func TestMapError_DuplicateAliasMessageIsSanitized(t *testing.T) {
	err := mapError(errors.New(`ERROR: duplicate key value violates unique constraint "idx_links_custom_alias" (SQLSTATE 23505)`))
	if msg := status.Convert(err).Message(); msg != "alias is already taken" {
		t.Errorf("message = %q, raw database error must not be exposed", msg)
	}
}

func TestCreateLink(t *testing.T) {
	svc := &fakeLinkService{link: testView()}
	userID := uuid.New()
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	res, err := NewLinkHandler(svc).CreateLink(context.Background(), &pb.CreateLinkRequest{
		UserId: userID.String(), OriginalUrl: "https://a.com", Alias: "my-alias", ExpiredAt: timestamppb.New(exp), Password: "pw",
	})
	if err != nil {
		t.Fatal(err)
	}
	in := svc.gotCreate
	if in.UserID != userID || in.Alias != "my-alias" || in.Password != "pw" || in.ExpiredAt == nil || !in.ExpiredAt.Equal(exp) {
		t.Errorf("service input = %+v", in)
	}
	if res.Link.Alias != "abc123" || res.Link.ClickCount != 5 || res.Link.ExpiredAt != nil {
		t.Errorf("link = %v", res.Link)
	}

	if _, err := NewLinkHandler(svc).CreateLink(context.Background(), &pb.CreateLinkRequest{UserId: userID.String()}); err != nil {
		t.Fatal(err)
	}
	if svc.gotCreate.ExpiredAt != nil {
		t.Error("missing expired_at must stay nil")
	}

	_, err = NewLinkHandler(svc).CreateLink(context.Background(), &pb.CreateLinkRequest{UserId: "bad"})
	wantCode(t, err, codes.InvalidArgument)
	_, err = NewLinkHandler(&fakeLinkService{err: errors.New("url domain is blocked")}).CreateLink(context.Background(), &pb.CreateLinkRequest{UserId: userID.String()})
	wantCode(t, err, codes.InvalidArgument)
}

func TestUpdateLink_ExpirySemantics(t *testing.T) {
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	linkID, userID := uuid.NewString(), uuid.NewString()
	base := &pb.UpdateLinkRequest{LinkId: linkID, UserId: userID}

	t.Run("expired_at set", func(t *testing.T) {
		svc := &fakeLinkService{link: testView()}
		req := &pb.UpdateLinkRequest{LinkId: linkID, UserId: userID, ExpiredAt: timestamppb.New(exp)}
		if _, err := NewLinkHandler(svc).UpdateLink(context.Background(), req); err != nil {
			t.Fatal(err)
		}
		if svc.gotUpdate.ClearExpiry || svc.gotUpdate.ExpiredAt == nil || !svc.gotUpdate.ExpiredAt.Equal(exp) {
			t.Errorf("input = %+v", svc.gotUpdate)
		}
	})
	t.Run("expired_at omitted clears expiry (PUT replace)", func(t *testing.T) {
		svc := &fakeLinkService{link: testView()}
		if _, err := NewLinkHandler(svc).UpdateLink(context.Background(), base); err != nil {
			t.Fatal(err)
		}
		if !svc.gotUpdate.ClearExpiry || svc.gotUpdate.ExpiredAt != nil {
			t.Errorf("input = %+v", svc.gotUpdate)
		}
	})
}

func TestIDValidation(t *testing.T) {
	good, bad := uuid.NewString(), "not-a-uuid"
	h := NewLinkHandler(&fakeLinkService{link: testView()})
	ctx := context.Background()

	calls := map[string]func(linkID, userID string) error{
		"GetLink": func(l, u string) error { _, err := h.GetLink(ctx, &pb.GetLinkRequest{LinkId: l, UserId: u}); return err },
		"UpdateLink": func(l, u string) error {
			_, err := h.UpdateLink(ctx, &pb.UpdateLinkRequest{LinkId: l, UserId: u})
			return err
		},
		"DeleteLink": func(l, u string) error {
			_, err := h.DeleteLink(ctx, &pb.DeleteLinkRequest{LinkId: l, UserId: u})
			return err
		},
	}
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			wantCode(t, call(bad, good), codes.InvalidArgument)
			wantCode(t, call(good, bad), codes.InvalidArgument)
			wantCode(t, call(good, good), codes.OK)
		})
	}

	_, err := h.ListLinks(ctx, &pb.ListLinksRequest{UserId: bad})
	wantCode(t, err, codes.InvalidArgument)
	_, err = h.GetUserLinkIDs(ctx, &pb.GetUserLinkIDsRequest{UserId: bad})
	wantCode(t, err, codes.InvalidArgument)
}

func TestGetAndDeleteLink_PassIDsAndMapErrors(t *testing.T) {
	linkID, userID := uuid.New(), uuid.New()
	svc := &fakeLinkService{link: testView()}
	if _, err := NewLinkHandler(svc).GetLink(context.Background(), &pb.GetLinkRequest{LinkId: linkID.String(), UserId: userID.String()}); err != nil {
		t.Fatal(err)
	}
	if svc.gotID != linkID || svc.gotUser != userID {
		t.Errorf("service got link=%s user=%s", svc.gotID, svc.gotUser)
	}

	notFound := &fakeLinkService{err: errors.New("link not found")}
	_, err := NewLinkHandler(notFound).GetLink(context.Background(), &pb.GetLinkRequest{LinkId: linkID.String(), UserId: userID.String()})
	wantCode(t, err, codes.NotFound)
	_, err = NewLinkHandler(notFound).DeleteLink(context.Background(), &pb.DeleteLinkRequest{LinkId: linkID.String(), UserId: userID.String()})
	wantCode(t, err, codes.NotFound)
	_, err = NewLinkHandler(notFound).UpdateLink(context.Background(), &pb.UpdateLinkRequest{LinkId: linkID.String(), UserId: userID.String()})
	wantCode(t, err, codes.NotFound)
}

func TestListLinks(t *testing.T) {
	exp := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	withExpiry := testView()
	withExpiry.ExpiredAt = &exp
	svc := &fakeLinkService{links: []*service.LinkView{testView(), withExpiry}, total: 42}

	res, err := NewLinkHandler(svc).ListLinks(context.Background(), &pb.ListLinksRequest{UserId: uuid.NewString(), Page: 2, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if svc.gotPage != 2 || svc.gotLimit != 10 || res.Total != 42 || len(res.Links) != 2 {
		t.Errorf("page=%d limit=%d res=%v", svc.gotPage, svc.gotLimit, res)
	}
	if res.Links[0].ExpiredAt != nil || !res.Links[1].ExpiredAt.AsTime().Equal(exp) {
		t.Error("expired_at not converted correctly")
	}

	_, err = NewLinkHandler(&fakeLinkService{err: errors.New("failed to list links: db down")}).ListLinks(context.Background(), &pb.ListLinksRequest{UserId: uuid.NewString()})
	wantCode(t, err, codes.Internal)
}

func TestResolveAlias(t *testing.T) {
	result := &service.ResolveResult{LinkID: uuid.New(), OriginalURL: "https://a.com", IsExpired: true, HasPassword: true}
	res, err := NewLinkHandler(&fakeLinkService{resolve: result}).ResolveAlias(context.Background(), &pb.ResolveAliasRequest{Alias: "abc"})
	if err != nil || res.LinkId != result.LinkID.String() || res.OriginalUrl != "https://a.com" || !res.IsExpired || !res.HasPassword {
		t.Errorf("res=%v err=%v", res, err)
	}

	_, err = NewLinkHandler(&fakeLinkService{err: errors.New("link not found")}).ResolveAlias(context.Background(), &pb.ResolveAliasRequest{})
	wantCode(t, err, codes.NotFound)
}

func TestVerifyLinkPassword(t *testing.T) {
	view := testView()
	res, err := NewLinkHandler(&fakeLinkService{link: view}).VerifyLinkPassword(context.Background(), &pb.VerifyLinkPasswordRequest{})
	if err != nil || !res.IsValid || res.OriginalUrl != view.OriginalURL || res.LinkId != view.ID.String() {
		t.Errorf("res=%v err=%v", res, err)
	}

	tests := map[string]codes.Code{
		"invalid password":               codes.Unauthenticated,
		"link has expired":               codes.FailedPrecondition,
		"link not found":                 codes.NotFound,
		"link is not password protected": codes.InvalidArgument,
	}
	for msg, want := range tests {
		t.Run(msg, func(t *testing.T) {
			_, err := NewLinkHandler(&fakeLinkService{err: errors.New(msg)}).VerifyLinkPassword(context.Background(), &pb.VerifyLinkPasswordRequest{})
			wantCode(t, err, want)
		})
	}
}

func TestGetUserLinkIDs(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	res, err := NewLinkHandler(&fakeLinkService{ids: ids}).GetUserLinkIDs(context.Background(), &pb.GetUserLinkIDsRequest{UserId: uuid.NewString()})
	if err != nil || len(res.LinkIds) != 2 || res.LinkIds[0] != ids[0].String() {
		t.Errorf("res=%v err=%v", res, err)
	}
	_, err = NewLinkHandler(&fakeLinkService{err: errors.New("db down")}).GetUserLinkIDs(context.Background(), &pb.GetUserLinkIDsRequest{UserId: uuid.NewString()})
	wantCode(t, err, codes.Internal)
}

func TestClientInfoFromMetadata(t *testing.T) {
	md := metadata.Pairs("x-forwarded-for", "203.0.113.7", "x-user-agent", "Mozilla/5.0", "x-referer", "https://twitter.com/")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	want := service.ClientInfo{IP: "203.0.113.7", UserAgent: "Mozilla/5.0", Referrer: "https://twitter.com/"}

	svc := &fakeLinkService{resolve: &service.ResolveResult{}, link: testView()}
	h := NewLinkHandler(svc)

	if _, err := h.ResolveAlias(ctx, &pb.ResolveAliasRequest{Alias: "abc"}); err != nil {
		t.Fatal(err)
	}
	if svc.gotClient != want {
		t.Errorf("ResolveAlias client = %+v, want %+v", svc.gotClient, want)
	}

	svc.gotClient = service.ClientInfo{}
	if _, err := h.VerifyLinkPassword(ctx, &pb.VerifyLinkPasswordRequest{Alias: "abc"}); err != nil {
		t.Fatal(err)
	}
	if svc.gotClient != want {
		t.Errorf("VerifyLinkPassword client = %+v, want %+v", svc.gotClient, want)
	}

	// Tanpa metadata (mis. dipanggil langsung, bukan via Gateway) → kosong, tidak panic
	if _, err := h.ResolveAlias(context.Background(), &pb.ResolveAliasRequest{Alias: "abc"}); err != nil {
		t.Fatal(err)
	}
	if svc.gotClient != (service.ClientInfo{}) {
		t.Errorf("client without metadata = %+v", svc.gotClient)
	}
}
