package app

import (
	"bufio"
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"MrFood/services/notification/config"
	"MrFood/services/notification/internal/api/grpc/pb"
	"MrFood/services/notification/internal/service"
	models "MrFood/services/notification/pkg"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeRedis struct {
	mu         sync.Mutex
	incrResult int64
	incrErr    error
	expireHits int
}

func (f *fakeRedis) Incr(_ context.Context, _ string) *redis.IntCmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	return redis.NewIntResult(f.incrResult, f.incrErr)
}

func (f *fakeRedis) Expire(_ context.Context, _ string, _ time.Duration) *redis.BoolCmd {
	f.mu.Lock()
	f.expireHits++
	f.mu.Unlock()
	return redis.NewBoolResult(true, nil)
}

func (f *fakeRedis) Close() error {
	return nil
}

func newTestServer(t *testing.T, redisClient *fakeRedis, smtpHost string, smtpPort int) *Server {
	t.Helper()

	svc := service.New(redisClient, service.SMTPConfig{
		Host: smtpHost,
		Port: smtpPort,
		From: "no-reply@mrfood.local",
	}, service.RateLimitConfig{
		EmailRateLimit: 1,
		RateLimitTTL:   time.Minute,
	})

	return &Server{svc: svc}
}

func startFakeSMTPServer(t *testing.T) (host string, port int, shutdown func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for fake SMTP server: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := lis.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					return
				}
			}

			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer c.Close()

				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(c)
				writer := bufio.NewWriter(c)

				if _, err := writer.WriteString("220 localhost ESMTP\r\n"); err != nil {
					return
				}
				if err := writer.Flush(); err != nil {
					return
				}

				inData := false
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}

					trimmed := strings.TrimSpace(line)
					upper := strings.ToUpper(trimmed)

					if inData {
						if trimmed == "." {
							if _, err := writer.WriteString("250 Message accepted\r\n"); err != nil {
								return
							}
							if err := writer.Flush(); err != nil {
								return
							}
							inData = false
						}
						continue
					}

					switch {
					case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
						_, err = writer.WriteString("250-localhost\r\n250-AUTH PLAIN\r\n250 OK\r\n")
					case strings.HasPrefix(upper, "AUTH PLAIN"):
						_, err = writer.WriteString("235 Authentication successful\r\n")
					case strings.HasPrefix(upper, "MAIL FROM:"), strings.HasPrefix(upper, "RCPT TO:"):
						_, err = writer.WriteString("250 OK\r\n")
					case upper == "DATA":
						inData = true
						_, err = writer.WriteString("354 End data with <CR><LF>.<CR><LF>\r\n")
					case upper == "RSET", upper == "NOOP":
						_, err = writer.WriteString("250 OK\r\n")
					case upper == "QUIT":
						_, err = writer.WriteString("221 Bye\r\n")
					default:
						_, err = writer.WriteString("250 OK\r\n")
					}

					if err != nil {
						return
					}
					if err := writer.Flush(); err != nil {
						return
					}
					if upper == "QUIT" {
						return
					}
				}
			}(conn)
		}
	}()

	shutdownFn := func() {
		close(stop)
		_ = lis.Close()
		wg.Wait()
	}

	addr := lis.Addr().(*net.TCPAddr)
	return "127.0.0.1", addr.Port, shutdownFn
}

func TestMapToGRPCError(t *testing.T) {
	tests := []struct {
		name     string
		in       error
		expected codes.Code
		msg      string
	}{
		{name: "invalid email", in: models.ErrInvalidEmail, expected: codes.InvalidArgument, msg: models.ErrInvalidEmail.Error()},
		{name: "invalid username", in: models.ErrInvalidUsername, expected: codes.InvalidArgument, msg: models.ErrInvalidUsername.Error()},
		{name: "empty receipts", in: models.ErrEmptyReceipts, expected: codes.InvalidArgument, msg: models.ErrEmptyReceipts.Error()},
		{name: "rate limit", in: models.ErrRateLimitExceeded, expected: codes.ResourceExhausted, msg: models.ErrRateLimitExceeded.Error()},
		{name: "send email failed", in: models.ErrSendEmailFailed, expected: codes.Internal, msg: models.ErrSendEmailFailed.Error()},
		{name: "default", in: errors.New("boom"), expected: codes.Internal, msg: "Internal server error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := mapToGRPCError(tc.in)
			if status.Code(err) != tc.expected {
				t.Fatalf("unexpected code: got %v want %v", status.Code(err), tc.expected)
			}
			if gotMsg := status.Convert(err).Message(); gotMsg != tc.msg {
				t.Fatalf("unexpected message: got %q want %q", gotMsg, tc.msg)
			}
		})
	}
}

func TestSendRegistrationEmail(t *testing.T) {
	host, port, stopSMTP := startFakeSMTPServer(t)
	defer stopSMTP()

	t.Run("invalid username", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, host, port)
		_, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{
			Email:    "user@example.com",
			Username: "",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, host, port)
		_, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{
			Email:    "",
			Username: "alice",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("send email failed", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, "127.0.0.1", 1)
		_, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{
			Email:    "user@example.com",
			Username: "alice",
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.Internal {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.Internal)
		}
	})

	t.Run("success", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, "localhost", port)
		resp, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{
			Email:    "user@example.com",
			Username: "alice",
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if resp == nil {
			t.Fatal("expected response, got nil")
		}
	})
}

func TestSendReceipts(t *testing.T) {
	host, port, stopSMTP := startFakeSMTPServer(t)
	defer stopSMTP()

	t.Run("empty receipts", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, host, port)
		_, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{
			Email:    "user@example.com",
			Receipts: nil,
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("invalid email", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{}, host, port)
		_, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{
			Email: "",
			Receipts: []*pb.Receipt{{
				Amount:               10,
				PaymentDescription:   "order #0",
				CurrentPaymentStatus: "paid",
				PaymentType:          "card",
				CreatedAt:            timestamppb.Now(),
			}},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.InvalidArgument)
		}
	})

	t.Run("rate limit exceeded", func(t *testing.T) {
		redisClient := &fakeRedis{incrResult: 2}
		s := newTestServer(t, redisClient, host, port)
		_, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{
			Email: "user@example.com",
			Receipts: []*pb.Receipt{{
				Amount:               10,
				PaymentDescription:   "order #1",
				CurrentPaymentStatus: "paid",
				PaymentType:          "card",
				CreatedAt:            timestamppb.Now(),
			}},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.ResourceExhausted {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.ResourceExhausted)
		}
	})

	t.Run("send email failed", func(t *testing.T) {
		s := newTestServer(t, &fakeRedis{incrResult: 1}, "127.0.0.1", 1)
		_, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{
			Email: "user@example.com",
			Receipts: []*pb.Receipt{{
				Amount:               20,
				PaymentDescription:   "order #2",
				CurrentPaymentStatus: "paid",
				PaymentType:          "card",
				CreatedAt:            timestamppb.Now(),
			}},
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if status.Code(err) != codes.Internal {
			t.Fatalf("unexpected code: got %v want %v", status.Code(err), codes.Internal)
		}
	})

	t.Run("success", func(t *testing.T) {
		redisClient := &fakeRedis{incrResult: 1}
		s := newTestServer(t, redisClient, "localhost", port)
		resp, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{
			Email: "user@example.com",
			Receipts: []*pb.Receipt{{
				Amount:               30,
				PaymentDescription:   "order #3",
				CurrentPaymentStatus: "paid",
				PaymentType:          "card",
				CreatedAt:            timestamppb.Now(),
			}},
		})
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if resp == nil {
			t.Fatal("expected response, got nil")
		}
		if redisClient.expireHits == 0 {
			t.Fatal("expected expire to be called on first rate limit increment")
		}
	})

}

func TestRunServerGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	app := &App{NotificationService: service.New(&fakeRedis{}, service.SMTPConfig{
		Host: "127.0.0.1",
		Port: 1,
		From: "no-reply@mrfood.local",
	}, service.RateLimitConfig{
		EmailRateLimit: 1,
		RateLimitTTL:   time.Minute,
	})}

	cfg := &config.Config{}
	cfg.Server.Port = 0

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.RunServer(ctx, cfg)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error from RunServer, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for RunServer to stop")
	}
}

func TestNewRedisClientError(t *testing.T) {
	cfg := &config.Config{}
	cfg.Redis.Host = "127.0.0.1"
	cfg.Redis.Port = 1

	client, err := newRedisClient(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if client != nil {
		t.Fatal("expected nil client when ping fails")
	}
}

func TestNewSMTPConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.SMTP.Host = "smtp.local"
	cfg.SMTP.Port = 2525
	cfg.SMTP.User = "user"
	cfg.SMTP.Password = "pass"
	cfg.SMTP.From = "no-reply@example.com"

	smtpCfg := newSMTPConfig(cfg)
	if smtpCfg.Host != cfg.SMTP.Host || smtpCfg.Port != cfg.SMTP.Port || smtpCfg.User != cfg.SMTP.User || smtpCfg.Password != cfg.SMTP.Password || smtpCfg.From != cfg.SMTP.From {
		t.Fatal("newSMTPConfig did not map fields correctly")
	}
}

func TestNewErrorWhenRedisUnavailable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Redis.Host = "127.0.0.1"
	cfg.Redis.Port = 1
	cfg.SMTP.Host = "localhost"
	cfg.SMTP.Port = 2525
	cfg.RateLimit.EmailRateLimit = 1
	cfg.RateLimit.TTL = time.Minute

	app, err := New(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if app != nil {
		t.Fatal("expected nil app when Redis is unavailable")
	}
}

func TestAppClose(t *testing.T) {
	app := &App{NotificationService: service.New(&fakeRedis{}, service.SMTPConfig{
		Host: "localhost",
		Port: 2525,
		From: "no-reply@mrfood.local",
	}, service.RateLimitConfig{
		EmailRateLimit: 1,
		RateLimitTTL:   time.Minute,
	})}

	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestNewRedisClientSuccess(t *testing.T) {
	mr := miniredis.RunT(t)

	host, portStr, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("failed to parse miniredis addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse miniredis port: %v", err)
	}

	cfg := &config.Config{}
	cfg.Redis.Host = host
	cfg.Redis.Port = port

	client, err := newRedisClient(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	_ = client.Close()
}

func TestNewSuccess(t *testing.T) {
	mr := miniredis.RunT(t)

	host, portStr, err := net.SplitHostPort(mr.Addr())
	if err != nil {
		t.Fatalf("failed to parse miniredis addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse miniredis port: %v", err)
	}

	cfg := &config.Config{}
	cfg.Redis.Host = host
	cfg.Redis.Port = port
	cfg.SMTP.Host = "localhost"
	cfg.SMTP.Port = 2525
	cfg.SMTP.User = "user"
	cfg.SMTP.Password = "pass"
	cfg.SMTP.From = "no-reply@example.com"
	cfg.RateLimit.EmailRateLimit = 2
	cfg.RateLimit.TTL = time.Minute

	app, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if app == nil || app.NotificationService == nil {
		t.Fatal("expected initialized app and notification service")
	}

	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("expected nil close error, got %v", err)
	}
}
