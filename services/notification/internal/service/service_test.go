package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"MrFood/services/notification/internal/api/grpc/pb"
	models "MrFood/services/notification/pkg"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeRedisClient struct {
	incrResult int64
	incrErr    error

	expireCalls int
	closeErr    error
}

func (f *fakeRedisClient) Incr(_ context.Context, _ string) *redis.IntCmd {
	return redis.NewIntResult(f.incrResult, f.incrErr)
}

func (f *fakeRedisClient) Expire(_ context.Context, _ string, _ time.Duration) *redis.BoolCmd {
	f.expireCalls++
	return redis.NewBoolResult(true, nil)
}

func (f *fakeRedisClient) Close() error {
	return f.closeErr
}

type fakeMailer struct {
	sendErr error

	calls   int
	to      string
	subject string
	body    string
}

func (m *fakeMailer) Send(to, subject, body string) error {
	m.calls++
	m.to = to
	m.subject = subject
	m.body = body
	return m.sendErr
}

func newServiceForTests(redisClient *fakeRedisClient, mailer Mailer) *Service {
	if mailer == nil {
		mailer = &fakeMailer{}
	}
	return &Service{redis: redisClient, mailer: mailer, rateLimit: RateLimitConfig{
		EmailRateLimit: 1,
		RateLimitTTL:   time.Minute,
	}}
}

func TestNewPanicsWhenRedisNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil redis client")
		}
	}()

	_ = New(nil, SMTPConfig{}, RateLimitConfig{})
}

func TestSendRegistrationEmailValidation(t *testing.T) {
	s := newServiceForTests(&fakeRedisClient{}, &fakeMailer{})

	_, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{Email: "a@b.com", Username: ""})
	if !errors.Is(err, models.ErrInvalidUsername) {
		t.Fatalf("expected ErrInvalidUsername, got %v", err)
	}

	_, err = s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{Email: "", Username: "alice"})
	if !errors.Is(err, models.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestSendRegistrationEmailSendFailAndSuccess(t *testing.T) {
	failingMailer := &fakeMailer{sendErr: errors.New("smtp down")}
	s := newServiceForTests(&fakeRedisClient{}, failingMailer)

	_, err := s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{Email: "a@b.com", Username: "alice"})
	if !errors.Is(err, models.ErrSendEmailFailed) {
		t.Fatalf("expected ErrSendEmailFailed, got %v", err)
	}

	successMailer := &fakeMailer{}
	s = newServiceForTests(&fakeRedisClient{}, successMailer)

	_, err = s.SendRegistrationEmail(context.Background(), &pb.SendRegistrationEmailRequest{Email: "a@b.com", Username: "alice"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if successMailer.calls != 1 {
		t.Fatalf("expected 1 mail call, got %d", successMailer.calls)
	}
	if successMailer.to != "a@b.com" {
		t.Fatalf("unexpected destination: %s", successMailer.to)
	}
	if successMailer.subject != "Welcome to MrFood!" {
		t.Fatalf("unexpected subject: %s", successMailer.subject)
	}
	if !strings.Contains(successMailer.body, "Hello alice") {
		t.Fatalf("unexpected body: %s", successMailer.body)
	}
}

func TestSendReceiptsValidationAndRateLimit(t *testing.T) {
	s := newServiceForTests(&fakeRedisClient{}, &fakeMailer{})

	_, err := s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{UserEmail: "", Receipts: []*pb.Receipt{{CreatedAt: timestamppb.Now()}}})
	if !errors.Is(err, models.ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}

	_, err = s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{UserEmail: "a@b.com", Receipts: nil})
	if !errors.Is(err, models.ErrEmptyReceipts) {
		t.Fatalf("expected ErrEmptyReceipts, got %v", err)
	}

	s = newServiceForTests(&fakeRedisClient{incrErr: errors.New("redis fail")}, &fakeMailer{})
	_, err = s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{UserEmail: "a@b.com", Receipts: []*pb.Receipt{{CreatedAt: timestamppb.Now()}}})
	if !errors.Is(err, models.ErrRedisIncrFailed) {
		t.Fatalf("expected ErrRedisIncrFailed, got %v", err)
	}

	s = newServiceForTests(&fakeRedisClient{incrResult: 2}, &fakeMailer{})
	_, err = s.SendReceipts(context.Background(), &pb.SendReceiptsRequest{UserEmail: "a@b.com", Receipts: []*pb.Receipt{{CreatedAt: timestamppb.Now()}}})
	if !errors.Is(err, models.ErrRateLimitExceeded) {
		t.Fatalf("expected ErrRateLimitExceeded, got %v", err)
	}
}

func TestSendReceiptsSendFailAndSuccess(t *testing.T) {
	fakeRedis := &fakeRedisClient{incrResult: 1}
	failingMailer := &fakeMailer{sendErr: errors.New("smtp down")}
	s := newServiceForTests(fakeRedis, failingMailer)

	req := &pb.SendReceiptsRequest{
		UserEmail: "a@b.com",
		Receipts: []*pb.Receipt{{
			Amount:               10.5,
			PaymentDescription:   "order-1",
			CurrentPaymentStatus: "paid",
			PaymentType:          "card",
			CreatedAt:            timestamppb.New(time.Date(2026, 4, 19, 10, 30, 0, 0, time.UTC)),
		}},
	}

	_, err := s.SendReceipts(context.Background(), req)
	if !errors.Is(err, models.ErrSendEmailFailed) {
		t.Fatalf("expected ErrSendEmailFailed, got %v", err)
	}

	successMailer := &fakeMailer{}
	s = newServiceForTests(fakeRedis, successMailer)

	_, err = s.SendReceipts(context.Background(), req)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if successMailer.calls != 1 {
		t.Fatalf("expected 1 mail call, got %d", successMailer.calls)
	}
	if successMailer.subject != "Your MrFood Receipts" {
		t.Fatalf("unexpected subject: %s", successMailer.subject)
	}
	if !strings.Contains(successMailer.body, "order-1") {
		t.Fatalf("expected receipt content in body: %s", successMailer.body)
	}
	if fakeRedis.expireCalls == 0 {
		t.Fatal("expected redis Expire to be called for first request")
	}
}

func TestClose(t *testing.T) {
	redisErr := errors.New("close failed")
	s := newServiceForTests(&fakeRedisClient{closeErr: redisErr}, &fakeMailer{})

	err := s.Close()
	if !errors.Is(err, redisErr) {
		t.Fatalf("expected closeErr, got %v", err)
	}
}

func TestNewInitializesSMTPMailer(t *testing.T) {
	s := New(&fakeRedisClient{}, SMTPConfig{}, RateLimitConfig{})
	if s == nil {
		t.Fatal("expected non-nil service")
	}
	if s.mailer == nil {
		t.Fatal("expected non-nil mailer")
	}
	if _, ok := s.mailer.(*SMTPMailer); !ok {
		t.Fatalf("expected SMTPMailer, got %T", s.mailer)
	}
}

func TestBuildReceiptsBody(t *testing.T) {
	body := buildReceiptsBody([]*pb.Receipt{{
		Amount:               12.34,
		PaymentDescription:   "sushi",
		CurrentPaymentStatus: "paid",
		PaymentType:          "card",
		CreatedAt:            timestamppb.New(time.Date(2026, 4, 19, 21, 45, 0, 0, time.UTC)),
	}})

	if !strings.Contains(body, "<h2>Your Receipts</h2>") {
		t.Fatalf("missing title in body: %s", body)
	}
	if !strings.Contains(body, "sushi") || !strings.Contains(body, "12.34") {
		t.Fatalf("missing row data in body: %s", body)
	}
	if !strings.Contains(body, "2026-04-19 21:45") {
		t.Fatalf("missing formatted date in body: %s", body)
	}
}
