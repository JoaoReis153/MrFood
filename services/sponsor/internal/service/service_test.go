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

func (m *MockRepo) GetRestaurantSponsorship(ctx context.Context, id int64) (*models.SponsorshipResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SponsorshipResponse), args.Error(1)
}

func (m *MockRepo) Sponsor(ctx context.Context, request *models.Sponsorship) (*models.SponsorshipResponse, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SponsorshipResponse), args.Error(1)
}

type MockRestaurantClient struct {
	mock.Mock
}

func (m *MockRestaurantClient) GetRestaurantSponsorship(
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

type MockPaymentClient struct {
	mock.Mock
}

func (m *MockPaymentClient) MakePayment(
	ctx context.Context,
	req *pb.PaymentRequest,
	opts ...grpc.CallOption,
) (*pb.PaymentResponse, error) {

	args := m.Called(ctx, req)

	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*pb.PaymentResponse), args.Error(1)
}

func TestSponsor_Success(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockRestaurant := new(MockRestaurantClient)
	mockPayment := new(MockPaymentClient)

	service := &Service{
		repo:             mockRepo,
		restaurantClient: mockRestaurant,
		paymentClient:    mockPayment,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	ownerID := int64(100)
	email := "test@test.com"

	mockRestaurant.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: req.ID,
	}).Return(&pb.GetRestaurantSponsorshipResponse{
		Id:      req.ID,
		OwnerId: ownerID,
	}, nil)

	expectedResp := &models.SponsorshipResponse{
		ID:   10,
		Tier: 2,
	}

	mockRepo.On("Sponsor", ctx, req).Return(expectedResp, nil)

	mockPayment.On("MakePayment", ctx, mock.Anything).
		Return(&pb.PaymentResponse{ReceiptId: 123}, nil)

	resp, receiptID, err := service.Sponsor(ctx, req, ownerID, email)

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	assert.Equal(t, int32(123), receiptID)

	mockRepo.AssertExpectations(t)
	mockRestaurant.AssertExpectations(t)
	mockPayment.AssertExpectations(t)
}

func TestSponsor_InvalidOwner(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockRestaurant := new(MockRestaurantClient)
	mockPayment := new(MockPaymentClient)

	service := &Service{
		repo:             mockRepo,
		restaurantClient: mockRestaurant,
		paymentClient:    mockPayment,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	mockRestaurant.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: req.ID,
	}).Return(&pb.GetRestaurantSponsorshipResponse{
		Id:      req.ID,
		OwnerId: 999,
	}, nil)

	resp, receiptID, err := service.Sponsor(ctx, req, 100, "test@test.com")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, int32(0), receiptID)
	assert.Equal(t, "invalid restaurant owner", err.Error())

	mockRepo.AssertNotCalled(t, "Sponsor")
	mockPayment.AssertNotCalled(t, "MakePayment")
}

func TestSponsor_ClientError(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)
	mockRestaurant := new(MockRestaurantClient)
	mockPayment := new(MockPaymentClient)

	service := &Service{
		repo:             mockRepo,
		restaurantClient: mockRestaurant,
		paymentClient:    mockPayment,
	}

	req := &models.Sponsorship{
		ID: 1,
	}

	mockRestaurant.On("GetRestaurantSponsorship", ctx, &pb.GetRestaurantSponsorshipRequest{
		Id: req.ID,
	}).Return(nil, errors.New("grpc error"))

	resp, receiptID, err := service.Sponsor(ctx, req, 100, "test@test.com")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, int32(0), receiptID)
	assert.Equal(t, "grpc error", err.Error())

	mockRepo.AssertNotCalled(t, "Sponsor")
	mockPayment.AssertNotCalled(t, "MakePayment")
}

func TestGetRestaurantSponsorship(t *testing.T) {
	ctx := context.Background()

	mockRepo := new(MockRepo)

	service := &Service{
		repo: mockRepo,
	}

	expected := &models.SponsorshipResponse{}

	mockRepo.On("GetRestaurantSponsorship", ctx, int64(1)).Return(expected, nil)

	resp, err := service.GetRestaurantSponsorship(ctx, 1)

	assert.NoError(t, err)
	assert.Equal(t, expected, resp)

	mockRepo.AssertExpectations(t)
}
