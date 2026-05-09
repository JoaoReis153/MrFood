package app

import (
	"context"
	"errors"
	"testing"
	"time"

	pb "MrFood/services/sponsor/internal/api/grpc/pb"
	models "MrFood/services/sponsor/pkg"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

// -----------------------------
// Mock Sponsor Service
// -----------------------------
type mockSponsorService struct {
	getFn     func(ctx context.Context, id int64) (*models.SponsorshipResponse, error)
	sponsorFn func(ctx context.Context, s *models.Sponsorship, userID int64, email string) (*models.SponsorshipResponse, int32, error)
}

func (m *mockSponsorService) GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
	return m.getFn(ctx, id)
}

func (m *mockSponsorService) Sponsor(ctx context.Context, s *models.Sponsorship, userID int64, email string) (*models.SponsorshipResponse, int32, error) {
	return m.sponsorFn(ctx, s, userID, email)
}

// -----------------------------
// JWT Helper
// -----------------------------
func createAuthContext(userID string, username string, email string) context.Context {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte("secret"))

	md := metadata.New(map[string]string{
		"authorization": "Bearer " + tokenStr,
	})

	return metadata.NewIncomingContext(context.Background(), md)
}

// -----------------------------
// GetRestaurantSponsorship Tests
// -----------------------------
func TestGetRestaurantSponsorship(t *testing.T) {
	mock := &mockSponsorService{
		getFn: func(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
			return &models.SponsorshipResponse{
				ID:    id,
				Tier:  2,
				Until: time.Now(),
			}, nil
		},
	}

	s := &server{sponsorService: mock}
	resp, err := s.GetRestaurantSponsorship(context.Background(), &pb.GetRestaurantSponsorshipRequest{Id: 10})

	require.NoError(t, err)
	require.Equal(t, int64(10), resp.Id)
	require.Equal(t, int32(2), resp.Tier)
}

func TestGetRestaurantSponsorship_Error(t *testing.T) {
	mock := &mockSponsorService{
		getFn: func(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
			return nil, errors.New("db error")
		},
	}

	s := &server{sponsorService: mock}
	resp, err := s.GetRestaurantSponsorship(context.Background(), &pb.GetRestaurantSponsorshipRequest{Id: 1})

	require.Error(t, err)
	require.Nil(t, resp)
}

// -----------------------------
// Sponsor Tests
// -----------------------------
func TestSponsor_Success(t *testing.T) {
	mock := &mockSponsorService{
		sponsorFn: func(ctx context.Context, s *models.Sponsorship, userID int64, email string) (*models.SponsorshipResponse, int32, error) {
			return &models.SponsorshipResponse{
				ID:    s.ID,
				Tier:  int(s.Tier),
				Until: s.Until,
			}, 123, nil
		},
	}

	s := &server{sponsorService: mock}
	ctx := createAuthContext("1", "john", "john@test.com")

	resp, err := s.Sponsor(ctx, &pb.SponsorshipRequest{Id: 5, Tier: 2})

	require.NoError(t, err)
	require.Equal(t, int64(5), resp.Id)
	require.Equal(t, int32(2), resp.Tier)
	require.Equal(t, int32(123), resp.ReceiptId)
}

func TestSponsor_InvalidTier(t *testing.T) {
	s := &server{}

	resp, err := s.Sponsor(context.Background(), &pb.SponsorshipRequest{Id: 1, Tier: 10})

	require.Error(t, err)
	require.Nil(t, resp)
}

func TestSponsor_NoAuth(t *testing.T) {
	s := &server{}

	resp, err := s.Sponsor(context.Background(), &pb.SponsorshipRequest{Id: 1, Tier: 2})

	require.Error(t, err)
	require.Nil(t, resp)
}

func TestSponsor_ServiceError(t *testing.T) {
	mock := &mockSponsorService{
		sponsorFn: func(ctx context.Context, s *models.Sponsorship, userID int64, email string) (*models.SponsorshipResponse, int32, error) {
			return nil, 0, errors.New("failed")
		},
	}

	s := &server{sponsorService: mock}
	ctx := createAuthContext("1", "john", "john@test.com")

	resp, err := s.Sponsor(ctx, &pb.SponsorshipRequest{Id: 1, Tier: 2})

	require.Error(t, err)
	require.Nil(t, resp)
}

// -----------------------------
// ExtractUserFromContext Tests
// -----------------------------
func TestExtractUserFromContext_Success(t *testing.T) {
	ctx := createAuthContext("42", "alice", "alice@test.com")

	user, err := ExtractUserFromContext(ctx)

	require.NoError(t, err)
	require.Equal(t, uuidToInt64("42"), user.UserID)
	require.Equal(t, "alice", user.Username)
	require.Equal(t, "alice@test.com", user.Email)
}

func TestExtractUserFromContext_NoMetadata(t *testing.T) {
	_, err := ExtractUserFromContext(context.Background())
	require.Error(t, err)
}

func TestExtractUserFromContext_InvalidToken(t *testing.T) {
	md := metadata.New(map[string]string{
		"authorization": "Bearer invalid.token.here",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := ExtractUserFromContext(ctx)
	require.Error(t, err)
}
