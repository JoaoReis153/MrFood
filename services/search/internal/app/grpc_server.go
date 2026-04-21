package app

import (
	"MrFood/services/search/config"
	pb "MrFood/services/search/internal/api/grpc/pb"
	"MrFood/services/search/internal/service"
	models "MrFood/services/search/pkg"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedSearchServiceServer
	service *service.Service
}

func newServer(svc *service.Service) *Server {
	return &Server{service: svc}
}

func (s *Server) SearchPaginated(ctx context.Context, req *pb.SearchPaginatedRequest) (*pb.SearchPaginatedResponse, error) {
	query := models.SearchQuery{
		Page:  req.GetPage(),
		Limit: req.GetLimit(),
	}

	if req.Category != nil {
		query.Filter.Category = req.Category
	}
	if req.NameSuffix != nil {
		query.Filter.NameSuffix = req.NameSuffix
	}
	if req.FullName != nil {
		query.Filter.FullName = req.FullName
	}
	if req.Latitude != nil || req.Longitude != nil || req.RadiusMeters != nil {
		if req.Latitude == nil || req.Longitude == nil || req.RadiusMeters == nil {
			return nil, status.Error(codes.InvalidArgument, "latitude, longitude and radius_meters must be provided together")
		}
		query.Filter.Location = &models.LocationRadius{
			Latitude:     req.GetLatitude(),
			Longitude:    req.GetLongitude(),
			RadiusMeters: req.GetRadiusMeters(),
		}
	}

	result, err := s.service.SearchPaginated(ctx, query)
	if err != nil {
		switch err {
		case service.ErrInvalidPagination, service.ErrInvalidGeoFilter, service.ErrInvalidTextFilter:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, "failed to search restaurants")
		}
	}

	resp := &pb.SearchPaginatedResponse{
		Pagination: &pb.Pagination{
			Page:  result.Pagination.Page,
			Limit: result.Pagination.Limit,
			Total: result.Pagination.Total,
			Pages: result.Pagination.Pages,
		},
	}

	for _, r := range result.Data {
		resp.Data = append(resp.Data, &pb.RestaurantSearchResult{
			Id:         r.ID,
			Name:       r.Name,
			Latitude:   r.Latitude,
			Longitude:  r.Longitude,
			Address:    r.Address,
			Categories: r.Categories,
			MediaUrl:   r.MediaURL,
		})
	}

	return resp, nil
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed", "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer()
	pb.RegisterSearchServiceServer(s, newServer(app.Service))

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("search", grpc_health_v1.HealthCheckResponse_SERVING)
	slog.Info("health check registered for service", "service", "search")

	slog.Info("gRPC server listening", "port", cfg.Server.Port)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := s.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-ctx.Done()
		slog.Info("shutting down gRPC server...")
		healthServer.SetServingStatus("search", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}
