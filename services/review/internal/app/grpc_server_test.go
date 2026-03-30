package app

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pb "MrFood/services/review/internal/api/grpc/pb"
	models "MrFood/services/review/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type mockReviewService struct {
	GetReviewsFn         func(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error)
	CreateReviewFn       func(ctx context.Context, review models.Review) (models.Review, error)
	UpdateReviewFn       func(ctx context.Context, review models.UpdateReview) (models.Review, error)
	DeleteReviewFn       func(ctx context.Context, deleteReq models.DeleteReview) error
	GetRestaurantStatsFn func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error)
}

func (m *mockReviewService) GetReviews(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error) {
	return m.GetReviewsFn(ctx, restaurantID, page, limit)
}
func (m *mockReviewService) CreateReview(ctx context.Context, review models.Review) (models.Review, error) {
	return m.CreateReviewFn(ctx, review)
}
func (m *mockReviewService) UpdateReview(ctx context.Context, review models.UpdateReview) (models.Review, error) {
	return m.UpdateReviewFn(ctx, review)
}
func (m *mockReviewService) DeleteReview(ctx context.Context, deleteReq models.DeleteReview) error {
	return m.DeleteReviewFn(ctx, deleteReq)
}
func (m *mockReviewService) GetRestaurantStats(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
	return m.GetRestaurantStatsFn(ctx, restaurantID)
}

func authenticatedContext(t *testing.T, userID string) context.Context {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{UserID: userID})
	tokenStr, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	md := metadata.Pairs("authorization", "Bearer "+tokenStr)
	return metadata.NewIncomingContext(context.Background(), md)
}

func TestServer_GetReviews_Success(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error) {
			if restaurantID != 5 || page != 2 || limit != 3 {
				t.Fatalf("unexpected args: id=%d page=%d limit=%d", restaurantID, page, limit)
			}
			return models.ReviewsPage{
				Reviews: []models.Review{
					{ReviewID: 1, RestaurantID: 5, UserID: 7, Rating: 4, Comment: "ok", CreatedAt: now},
				},
				Pagination: models.Pagination{
					Page:  2,
					Limit: 3,
					Total: 1,
					Pages: 1,
				},
			}, nil
		},
	}

	s := &server{svc: ms}

	page := int32(2)
	limit := int32(3)
	resp, err := s.GetReviews(ctx, &pb.GetReviewsRequest{
		RestaurantId: 5,
		Page:         &page,
		Limit:        &limit,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(resp.Reviews))
	}
	r := resp.Reviews[0]
	if r.ReviewId != 1 || r.RestaurantId != 5 || r.UserId != 7 || r.Rating != 4 || r.Comment != "ok" {
		t.Fatalf("unexpected review: %+v", r)
	}
	if resp.Pagination.Page != 2 || resp.Pagination.Limit != 3 || resp.Pagination.Total != 1 || resp.Pagination.Pages != 1 {
		t.Fatalf("unexpected pagination: %+v", resp.Pagination)
	}
	if !r.CreatedAt.AsTime().Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, r.CreatedAt.AsTime())
	}
}

func TestServer_GetReviews_DefaultsAndError(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error) {
			if page != 1 || limit != 10 {
				t.Fatalf("expected default page=1 limit=10, got page=%d limit=%d", page, limit)
			}
			return models.ReviewsPage{}, models.ErrRestaurantNotFound
		},
	}

	s := &server{svc: ms}

	_, err := s.GetReviews(ctx, &pb.GetReviewsRequest{
		RestaurantId: 5,
		// Page=0, Limit=0
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st)
	}
}

func TestServer_CreateReview_Success(t *testing.T) {
	ctx := authenticatedContext(t, "2")
	now := time.Now()

	ms := &mockReviewService{
		CreateReviewFn: func(ctx context.Context, review models.Review) (models.Review, error) {
			if review.RestaurantID != 1 || review.UserID != 2 || review.Rating != 5 || review.Comment != "good" {
				t.Fatalf("unexpected review in service: %+v", review)
			}
			review.ReviewID = 10
			review.CreatedAt = now
			return review, nil
		},
	}
	s := &server{svc: ms}

	resp, err := s.CreateReview(ctx, &pb.CreateReviewRequest{
		RestaurantId: 1,
		Rating:       5,
		Comment:      "good",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := resp.Review
	if r.ReviewId != 10 || r.RestaurantId != 1 || r.UserId != 2 || r.Rating != 5 || r.Comment != "good" {
		t.Fatalf("unexpected review resp: %+v", r)
	}
	if !r.CreatedAt.AsTime().Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, r.CreatedAt.AsTime())
	}
}

func TestServer_CreateReview_Error(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		CreateReviewFn: func(ctx context.Context, review models.Review) (models.Review, error) {
			return models.Review{}, models.ErrReviewAlreadyExists
		},
	}
	s := &server{svc: ms}

	_, err := s.CreateReview(ctx, &pb.CreateReviewRequest{RestaurantId: 1, Rating: 3})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", st.Code())
	}
}

func TestServer_UpdateReview_Success(t *testing.T) {
	ctx := authenticatedContext(t, "2")
	now := time.Now()

	ms := &mockReviewService{
		UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
			if review.ReviewID != 10 {
				t.Fatalf("expected ReviewID 10, got %d", review.ReviewID)
			}
			if review.Comment == nil || *review.Comment != "new" {
				t.Fatalf("unexpected comment: %v", review.Comment)
			}
			if review.Rating == nil || *review.Rating != 4 {
				t.Fatalf("unexpected rating: %v", review.Rating)
			}
			return models.Review{
				ReviewID:     10,
				RestaurantID: 1,
				UserID:       2,
				Rating:       4,
				Comment:      "new",
				CreatedAt:    now,
			}, nil
		},
	}
	s := &server{svc: ms}

	comment := "new"
	rating := int32(4)
	resp, err := s.UpdateReview(ctx, &pb.UpdateReviewRequest{
		ReviewId: 10,
		Comment:  &comment,
		Rating:   &rating,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := resp.Review
	if r.ReviewId != 10 || r.RestaurantId != 1 || r.UserId != 2 || r.Rating != 4 || r.Comment != "new" {
		t.Fatalf("unexpected response: %+v", r)
	}
	if !r.CreatedAt.AsTime().Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, r.CreatedAt.AsTime())
	}
}

func TestServer_UpdateReview_Error(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
			return models.Review{}, models.ErrReviewNotFound
		},
	}
	s := &server{svc: ms}

	_, err := s.UpdateReview(ctx, &pb.UpdateReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestServer_DeleteReview_Success(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		DeleteReviewFn: func(ctx context.Context, deleteReq models.DeleteReview) error {
			if deleteReq.ReviewID != 7 || deleteReq.UserID != 2 {
				t.Fatalf("unexpected delete request: %+v", deleteReq)
			}
			return nil
		},
	}
	s := &server{svc: ms}

	resp, err := s.DeleteReview(ctx, &pb.DeleteReviewRequest{ReviewId: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
}

func TestServer_DeleteReview_Error(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		DeleteReviewFn: func(ctx context.Context, deleteReq models.DeleteReview) error {
			return models.ErrReviewNotFound
		},
	}
	s := &server{svc: ms}

	_, err := s.DeleteReview(ctx, &pb.DeleteReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestServer_GetRestaurantStats_Success(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
			if restaurantID != 5 {
				t.Fatalf("expected id 5, got %d", restaurantID)
			}
			return models.RestaurantStats{
				RestaurantID:  5,
				AverageRating: 4.5,
				ReviewCount:   12,
			}, nil
		},
	}
	s := &server{svc: ms}

	resp, err := s.GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rs := resp.RestaurantStats
	if rs.RestaurantId != 5 || rs.AverageRating != 4.5 || rs.ReviewCount != 12 {
		t.Fatalf("unexpected stats: %+v", rs)
	}
}

func TestServer_GetRestaurantStats_Error(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
			return models.RestaurantStats{}, errors.New("db")
		},
	}
	s := &server{svc: ms}

	_, err := s.GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: 5})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

func TestMapToGRPCError_AllCases(t *testing.T) {
	cases := []struct {
		err      error
		expected codes.Code
	}{
		{models.ErrInvalidRating, codes.InvalidArgument},
		{models.ErrInvalidComment, codes.InvalidArgument},
		{models.ErrInvalidRestaurantID, codes.InvalidArgument},
		{models.ErrInvalidUserID, codes.InvalidArgument},
		{models.ErrInvalidReviewID, codes.InvalidArgument},
		{models.ErrLimitTooLarge, codes.InvalidArgument},
		{models.ErrReviewAlreadyExists, codes.AlreadyExists},
		{models.ErrRestaurantNotFound, codes.NotFound},
		{models.ErrReviewNotFound, codes.NotFound},
		{errors.New("other"), codes.Internal},
	}

	for _, tc := range cases {
		st, ok := status.FromError(mapToGRPCError(tc.err))
		if !ok {
			t.Fatalf("expected status error for %v", tc.err)
		}
		if st.Code() != tc.expected {
			t.Fatalf("for error %v expected code %v got %v", tc.err, tc.expected, st.Code())
		}
	}
}

const bufSize = 1024 * 1024

func TestRunServer_Smoke(t *testing.T) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	pb.RegisterReviewServiceServer(s, &server{
		svc: &mockReviewService{
			GetReviewsFn: func(ctx context.Context, restaurantID, page, limit int) (models.ReviewsPage, error) {
				return models.ReviewsPage{
					Reviews: []models.Review{},
					Pagination: models.Pagination{
						Page:  1,
						Limit: 10,
						Total: 0,
						Pages: 0,
					},
				}, nil
			},
			CreateReviewFn: func(ctx context.Context, review models.Review) (models.Review, error) { return review, nil },
			UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
				return models.Review{}, nil
			},
			DeleteReviewFn: func(ctx context.Context, deleteReq models.DeleteReview) error { return nil },
			GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
				return models.RestaurantStats{}, nil
			},
		},
	})
	go func() {
		_ = s.Serve(lis)
	}()
	defer s.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	client := pb.NewReviewServiceClient(conn)

	page := int32(1)
	limit := int32(10)
	_, err = client.GetReviews(ctx, &pb.GetReviewsRequest{RestaurantId: 1, Page: &page, Limit: &limit})
	if err != nil {
		t.Fatalf("unexpected error calling GetReviews over bufconn: %v", err)
	}
}
