package service

import (
	"MrFood/services/sponsor/internal/api/grpc/pb"
	models "MrFood/services/sponsor/pkg"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
)

type MockRepo struct {
	mock.Mock
}

func (m *MockRepo) GetRestaurantSponsorship(ctx context.Context, id int32) (*models.SponsorshipResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SponsorshipResponse), args.Error(1)
}

type MockClient struct {
	mock.Mock
}

func (m *MockClient) GetRestaurantSponsorship(
	ctx context.Context,
	req *pb.GetRestaurantSponsorshipRequest,
	opts ...grpc.CallOption,
) (*pb.GetRestaurantSponsorshipResponse, error) {

	args := m.Called(ctx, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*pb.GetRestaurantSponsorshipResponse), args.Error(1)
}

func (m *MockRepo) Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SponsorshipResponse), args.Error(1)
}

func TestSponsor_Success(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockClient := new(MockClient)

	service := &Service{
		repo:   mockRepo,
		client: mockClient,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	ownerID := 100

	mockClient.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: int32(req.ID),
	}).Return(&pb.GetRestaurantSponsorshipResponse{
		Id:      int32(req.ID),
		OwnerId: int32(ownerID),
	}, nil)

	expectedResp := &models.SponsorshipResponse{}

	mockRepo.On("Sponsor", ctx, req).Return(expectedResp, nil)

	resp, err := service.Sponsor(ctx, req, ownerID)

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)

	mockRepo.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestSponsor_InvalidOwner(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockClient := new(MockClient)

	service := &Service{
		repo:   mockRepo,
		client: mockClient,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	mockClient.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: int32(req.ID),
	}).Return(&pb.GetRestaurantSponsorshipResponse{
		Id:      int32(req.ID),
		OwnerId: 999, // different owner
	}, nil)

	resp, err := service.Sponsor(ctx, req, 100)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "Invalid restaurant owner", err.Error())

	mockRepo.AssertNotCalled(t, "Sponsor")
}

func TestSponsor_ClientError(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockClient := new(MockClient)

	service := &Service{
		repo:   mockRepo,
		client: mockClient,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	mockClient.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: int32(req.ID),
	}).Return(nil, errors.New("grpc error"))

	resp, err := service.Sponsor(ctx, req, 100)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, "grpc error", err.Error())

	mockRepo.AssertNotCalled(t, "Sponsor")
}

func TestGetRestaurantSponsorship(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockClient := new(MockClient)

	service := &Service{
		repo:   mockRepo,
		client: mockClient,
	}

	expected := &models.SponsorshipResponse{}

	mockRepo.On("GetRestaurantSponsorship", ctx, int32(1)).Return(expected, nil)

	resp, err := service.GetRestaurantSponsorship(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expected, resp)

	mockRepo.AssertExpectations(t)
}
