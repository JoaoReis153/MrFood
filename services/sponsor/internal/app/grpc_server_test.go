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
	getFn     func(ctx context.Context, id int32) (*models.SponsorshipResponse, error)
	sponsorFn func(ctx context.Context, s *models.Sponsorship, userID int) (*models.SponsorshipResponse, error)
}

func (m *mockSponsorService) GetRestaurantSponsorship(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
	return m.getFn(ctx, id)
}

func (m *mockSponsorService) Sponsor(ctx context.Context, s *models.Sponsorship, userID int) (*models.SponsorshipResponse, error) {
	return m.sponsorFn(ctx, s, userID)
}

// -----------------------------
// JWT Helper
// -----------------------------
func createAuthContext(userID string, username string) context.Context {
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
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
		getFn: func(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
			return &models.SponsorshipResponse{
				ID:    int(id),
				Tier:  2,
				Until: time.Now(),
			}, nil
		},
	}

	s := &server{sponsorService: mock}
	resp, err := s.GetRestaurantSponsorship(context.Background(), &pb.GetRestaurantSponsorshipRequest{Id: 10})
	require.NoError(t, err)
	require.Equal(t, int32(10), resp.Id)
	require.Equal(t, int32(2), resp.Tier)
}

func TestGetRestaurantSponsorship_Error(t *testing.T) {
	mock := &mockSponsorService{
		getFn: func(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
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
		sponsorFn: func(ctx context.Context, s *models.Sponsorship, userID int) (*models.SponsorshipResponse, error) {
			return &models.SponsorshipResponse{
				ID:    int(s.ID),
				Tier:  int(s.Tier),
				Until: s.Until,
			}, nil
		},
	}

	s := &server{sponsorService: mock}
	ctx := createAuthContext("1", "john")
	resp, err := s.Sponsor(ctx, &pb.SponsorshipRequest{Id: 5, Tier: 2})
	require.NoError(t, err)
	require.Equal(t, int32(5), resp.Id)
	require.Equal(t, int32(2), resp.Tier)
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
		sponsorFn: func(ctx context.Context, s *models.Sponsorship, userID int) (*models.SponsorshipResponse, error) {
			return nil, errors.New("failed")
		},
	}

	s := &server{sponsorService: mock}
	ctx := createAuthContext("1", "john")
	resp, err := s.Sponsor(ctx, &pb.SponsorshipRequest{Id: 1, Tier: 2})
	require.Error(t, err)
	require.Nil(t, resp)
}

// -----------------------------
// ExtractUserFromContext Tests
// -----------------------------
func TestExtractUserFromContext_Success(t *testing.T) {
	ctx := createAuthContext("42", "alice")
	user, err := ExtractUserFromContext(ctx)
	require.NoError(t, err)
	require.Equal(t, int32(42), user.UserID)
	require.Equal(t, "alice", user.Username)
}

func TestExtractUserFromContext_NoMetadata(t *testing.T) {
	_, err := ExtractUserFromContext(context.Background())
	require.Error(t, err)
}

func TestExtractUserFromContext_InvalidToken(t *testing.T) {
	md := metadata.New(map[string]string{"authorization": "Bearer invalid.token.here"})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := ExtractUserFromContext(ctx)
	require.Error(t, err)
}
