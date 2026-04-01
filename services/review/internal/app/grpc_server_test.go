package app

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/review/internal/api/grpc/pb"
	models "MrFood/services/review/pkg"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ===================================================
// ==================== Mock =======================
// ===================================================

type mockReviewService struct {
	GetReviewsFn         func(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error)
	CreateReviewFn       func(ctx context.Context, review models.Review) (models.Review, error)
	UpdateReviewFn       func(ctx context.Context, review models.UpdateReview) (models.Review, error)
	DeleteReviewFn       func(ctx context.Context, deleteReq models.DeleteReview) error
	GetRestaurantStatsFn func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error)
}

func (m *mockReviewService) GetReviews(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error) {
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

// ===================================================
// ================== Helpers ========================
// ===================================================

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

func unauthenticatedContext() context.Context {
	return context.Background()
}

func newServer(svc ReviewService) *server {
	return &server{svc: svc}
}

// ===================================================
// ================ GetReviews =======================
// ===================================================

func TestServer_GetReviews_Success(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error) {
			if restaurantID != 5 || page != 2 || limit != 3 {
				t.Fatalf("unexpected args: id=%d page=%d limit=%d", restaurantID, page, limit)
			}
			return models.ReviewsPage{
				Reviews: []models.Review{
					{ReviewID: 1, RestaurantID: 5, UserID: 7, Rating: 4, Comment: "ok", CreatedAt: now},
				},
				Pagination: models.Pagination{Page: 2, Limit: 3, Total: 1, Pages: 1},
			}, nil
		},
	}

	page := int32(2)
	limit := int32(3)
	resp, err := newServer(ms).GetReviews(ctx, &pb.GetReviewsRequest{
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
	if !r.CreatedAt.AsTime().Equal(now.UTC().Truncate(time.Second)) && !r.CreatedAt.AsTime().Equal(now) {
		t.Fatalf("expected CreatedAt %v, got %v", now, r.CreatedAt.AsTime())
	}
}

func TestServer_GetReviews_Defaults(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error) {
			if page != 1 || limit != 10 {
				t.Fatalf("expected default page=1 limit=10, got page=%d limit=%d", page, limit)
			}
			return models.ReviewsPage{
				Reviews:    []models.Review{},
				Pagination: models.Pagination{Page: 1, Limit: 10, Total: 0, Pages: 0},
			}, nil
		},
	}

	resp, err := newServer(ms).GetReviews(ctx, &pb.GetReviewsRequest{RestaurantId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Reviews) != 0 {
		t.Fatalf("expected 0 reviews, got %d", len(resp.Reviews))
	}
}

func TestServer_GetReviews_NotFound(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error) {
			return models.ReviewsPage{}, models.ErrRestaurantNotFound
		},
	}

	_, err := newServer(ms).GetReviews(ctx, &pb.GetReviewsRequest{RestaurantId: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st)
	}
}

func TestServer_GetReviews_InvalidArgument(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetReviewsFn: func(ctx context.Context, restaurantID int32, page, limit int) (models.ReviewsPage, error) {
			return models.ReviewsPage{}, models.ErrInvalidRestaurantID
		},
	}

	_, err := newServer(ms).GetReviews(ctx, &pb.GetReviewsRequest{RestaurantId: -1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// ===================================================
// ================ CreateReview =====================
// ===================================================

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

	resp, err := newServer(ms).CreateReview(ctx, &pb.CreateReviewRequest{
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
}

func TestServer_CreateReview_Unauthenticated(t *testing.T) {
	ms := &mockReviewService{}

	_, err := newServer(ms).CreateReview(unauthenticatedContext(), &pb.CreateReviewRequest{
		RestaurantId: 1,
		Rating:       5,
		Comment:      "good",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied && st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated or PermissionDenied, got %v", st.Code())
	}
}

func TestServer_CreateReview_AlreadyExists(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		CreateReviewFn: func(ctx context.Context, review models.Review) (models.Review, error) {
			return models.Review{}, models.ErrReviewAlreadyExists
		},
	}

	_, err := newServer(ms).CreateReview(ctx, &pb.CreateReviewRequest{RestaurantId: 1, Rating: 3, Comment: "ok"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists, got %v", st.Code())
	}
}

func TestServer_CreateReview_ServiceError(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		CreateReviewFn: func(ctx context.Context, review models.Review) (models.Review, error) {
			return models.Review{}, models.ErrInvalidRating
		},
	}

	_, err := newServer(ms).CreateReview(ctx, &pb.CreateReviewRequest{RestaurantId: 1, Rating: 0, Comment: "ok"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// ===================================================
// ================ UpdateReview =====================
// ===================================================

func TestServer_UpdateReview_Success(t *testing.T) {
	ctx := authenticatedContext(t, "2")
	now := time.Now()

	ms := &mockReviewService{
		UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
			if review.ReviewID != 10 || review.UserID != 2 {
				t.Fatalf("unexpected update request: %+v", review)
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

	comment := "new"
	rating := int32(4)
	resp, err := newServer(ms).UpdateReview(ctx, &pb.UpdateReviewRequest{
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
}

func TestServer_UpdateReview_PartialUpdate(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
			if review.Rating != nil {
				t.Fatal("expected nil rating")
			}
			if review.Comment == nil || *review.Comment != "updated" {
				t.Fatalf("unexpected comment: %v", review.Comment)
			}
			return models.Review{ReviewID: 1, Comment: "updated"}, nil
		},
	}

	comment := "updated"
	resp, err := newServer(ms).UpdateReview(ctx, &pb.UpdateReviewRequest{
		ReviewId: 1,
		Comment:  &comment,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Review.Comment != "updated" {
		t.Fatalf("expected comment 'updated', got %s", resp.Review.Comment)
	}
}

func TestServer_UpdateReview_Unauthenticated(t *testing.T) {
	ms := &mockReviewService{}

	_, err := newServer(ms).UpdateReview(unauthenticatedContext(), &pb.UpdateReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied && st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated or PermissionDenied, got %v", st.Code())
	}
}

func TestServer_UpdateReview_NotFound(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		UpdateReviewFn: func(ctx context.Context, review models.UpdateReview) (models.Review, error) {
			return models.Review{}, models.ErrReviewNotFound
		},
	}

	_, err := newServer(ms).UpdateReview(ctx, &pb.UpdateReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

// ===================================================
// ================ DeleteReview =====================
// ===================================================

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

	resp, err := newServer(ms).DeleteReview(ctx, &pb.DeleteReviewRequest{ReviewId: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
}

func TestServer_DeleteReview_Unauthenticated(t *testing.T) {
	ms := &mockReviewService{}

	_, err := newServer(ms).DeleteReview(unauthenticatedContext(), &pb.DeleteReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied && st.Code() != codes.Unauthenticated {
		t.Fatalf("expected Unauthenticated or PermissionDenied, got %v", st.Code())
	}
}

func TestServer_DeleteReview_NotFound(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		DeleteReviewFn: func(ctx context.Context, deleteReq models.DeleteReview) error {
			return models.ErrReviewNotFound
		},
	}

	_, err := newServer(ms).DeleteReview(ctx, &pb.DeleteReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestServer_DeleteReview_Forbidden(t *testing.T) {
	ctx := authenticatedContext(t, "2")

	ms := &mockReviewService{
		DeleteReviewFn: func(ctx context.Context, deleteReq models.DeleteReview) error {
			return models.ErrForbidden
		},
	}

	_, err := newServer(ms).DeleteReview(ctx, &pb.DeleteReviewRequest{ReviewId: 1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.PermissionDenied {
		t.Fatalf("expected PermissionDenied, got %v", st.Code())
	}
}

// ===================================================
// ============= GetRestaurantStats ==================
// ===================================================

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

	resp, err := newServer(ms).GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rs := resp.RestaurantStats
	if rs.RestaurantId != 5 || rs.AverageRating != 4.5 || rs.ReviewCount != 12 {
		t.Fatalf("unexpected stats: %+v", rs)
	}
}

func TestServer_GetRestaurantStats_NotFound(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
			return models.RestaurantStats{}, models.ErrRestaurantNotFound
		},
	}

	_, err := newServer(ms).GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", st.Code())
	}
}

func TestServer_GetRestaurantStats_Internal(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
			return models.RestaurantStats{}, errors.New("db error")
		},
	}

	_, err := newServer(ms).GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", st.Code())
	}
}

func TestServer_GetRestaurantStats_InvalidArgument(t *testing.T) {
	ctx := context.Background()

	ms := &mockReviewService{
		GetRestaurantStatsFn: func(ctx context.Context, restaurantID int32) (models.RestaurantStats, error) {
			return models.RestaurantStats{}, models.ErrInvalidRestaurantID
		},
	}

	_, err := newServer(ms).GetRestaurantStats(ctx, &pb.GetRestaurantStatsRequest{RestaurantId: -1})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// ===================================================
// ================ mapToGRPCError ===================
// ===================================================

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
		{models.ErrForbidden, codes.PermissionDenied},
		{models.ErrUnauthenticated, codes.Unauthenticated},
		{models.ErrRestaurantServiceUnavailable, codes.Unavailable},
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

// ===================================================
// ================ ExtractUser ======================
// ===================================================

func TestExtractUserFromContext_NoMetadata(t *testing.T) {
	_, err := ExtractUserFromContext(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractUserFromContext_NoAuthHeader(t *testing.T) {
	md := metadata.Pairs("other-header", "value")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := ExtractUserFromContext(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExtractUserFromContext_ValidToken(t *testing.T) {
	ctx := authenticatedContext(t, "42")
	claims, err := ExtractUserFromContext(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if claims.UserID != "42" {
		t.Fatalf("expected userID 42, got %s", claims.UserID)
	}
}

// ===================================================
// ================= parseInt32 ======================
// ===================================================

func TestParseInt32_Valid(t *testing.T) {
	v, err := parseInt32("42")
	if err != nil || v != 42 {
		t.Fatalf("expected 42, got %d err %v", v, err)
	}
}

func TestParseInt32_Zero(t *testing.T) {
	_, err := parseInt32("0")
	if err == nil {
		t.Fatal("expected error for 0, got nil")
	}
}

func TestParseInt32_Negative(t *testing.T) {
	_, err := parseInt32("-1")
	if err == nil {
		t.Fatal("expected error for negative, got nil")
	}
}

func TestParseInt32_NonNumeric(t *testing.T) {
	_, err := parseInt32("abc")
	if err == nil {
		t.Fatal("expected error for non-numeric, got nil")
	}
}
