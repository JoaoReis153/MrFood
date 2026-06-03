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

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
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
	page := req.GetPage()
	if page <= 0 {
		page = 1
	}
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 10
	}

	slog.InfoContext(ctx, "search request", "page", page, "limit", limit, "category", req.GetCategory(), "name_suffix", req.GetNameSuffix(), "full_name", req.GetFullName(), "lat", req.GetLatitude(), "lon", req.GetLongitude(), "radius_meters", req.GetRadiusMeters())

	query := models.SearchQuery{
		Page:  page,
		Limit: limit,
	}

	if req.GetCategory() != "" {
		cat := req.GetCategory()
		query.Filter.Category = &cat
	}
	if req.GetNameSuffix() != "" {
		suffix := req.GetNameSuffix()
		query.Filter.NameSuffix = &suffix
	}
	if req.GetFullName() != "" {
		full := req.GetFullName()
		query.Filter.FullName = &full
	}

	if req.GetLatitude() != 0 || req.GetLongitude() != 0 {
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
			slog.WarnContext(ctx, "search rejected: invalid params", "error", err)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			slog.ErrorContext(ctx, "search failed", "error", err)
			return nil, status.Error(codes.Internal, "failed to search restaurants")
		}
	}

	slog.InfoContext(ctx, "search completed", "page", result.Pagination.Page, "limit", result.Pagination.Limit, "total", result.Pagination.Total, "results", len(result.Data))

	resp := &pb.SearchPaginatedResponse{
		Pagination: &pb.Pagination{
			Page:  result.Pagination.Page,
			Limit: result.Pagination.Limit,
			Total: result.Pagination.Total,
			Pages: result.Pagination.Pages,
		},
		Data: make([]*pb.RestaurantSearchResult, 0, len(result.Data)),
	}

	for _, r := range result.Data {
		// Handle the optional media_url safely
		var mediaURL string
		if r.MediaURL != nil {
			mediaURL = *r.MediaURL
		}

		resp.Data = append(resp.Data, &pb.RestaurantSearchResult{
			Id:         r.ID,
			Name:       r.Name,
			Latitude:   r.Latitude,
			Longitude:  r.Longitude,
			Address:    r.Address,
			Categories: r.Categories,
			MediaUrl:   mediaURL,
		})
	}

	return resp, nil
}

func (app *App) RunServer(ctx context.Context, cfg *config.Config) error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Server.Port))
	if err != nil {
		slog.Error("failed to listen", "port", cfg.Server.Port, "error", err)
		os.Exit(1)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)
	pb.RegisterSearchServiceServer(s, newServer(app.Service))

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
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
		healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.GracefulStop()
		healthServer.Shutdown()
		return nil
	})

	return g.Wait()
}
