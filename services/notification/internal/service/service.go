package service

import (
	"MrFood/services/notification/internal/api/grpc/pb"
	models "MrFood/services/notification/pkg"
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Close() error
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type Service struct {
	redis     redisClient
	smtp      SMTPConfig
	rateLimit RateLimitConfig
}

type RateLimitConfig struct {
	EmailRateLimit int
	RateLimitTTL   time.Duration
}

func New(redis redisClient, smtp SMTPConfig, rateLimit RateLimitConfig) *Service {
	if redis == nil {
		panic("nil redis client")
	}
	return &Service{redis: redis, smtp: smtp, rateLimit: rateLimit}
}

func (s *Service) SendRegistrationEmail(
	ctx context.Context,
	userInfo *pb.SendRegistrationEmailRequest) (*pb.SendRegistrationEmailResponse, error) {

	if userInfo.Username == "" {
		return &pb.SendRegistrationEmailResponse{}, models.ErrInvalidUsername
	}
	if userInfo.Email == "" {
		return &pb.SendRegistrationEmailResponse{}, models.ErrInvalidEmail
	}

	err := s.sendEmail(
		userInfo.Email,
		"Welcome to MrFood!",
		fmt.Sprintf("Hello %s,<br><br>Thank you for registering at MrFood! We're excited to have you on board.<br><br>Best regards,<br>MrFood Team", userInfo.Username),
	)
	if err != nil {
		return &pb.SendRegistrationEmailResponse{}, models.ErrSendEmailFailed
	}

	return &pb.SendRegistrationEmailResponse{}, nil
}

func (s *Service) SendReceipts(
	ctx context.Context,
	receiptInfo *pb.SendReceiptsRequest) (*pb.SendReceiptsResponse, error) {

	if receiptInfo.Email == "" {
		return &pb.SendReceiptsResponse{}, models.ErrInvalidEmail
	}
	if len(receiptInfo.Receipts) == 0 {
		return &pb.SendReceiptsResponse{}, models.ErrEmptyReceipts
	}

	if err := s.checkRateLimit(ctx, receiptInfo.Email); err != nil {
		return &pb.SendReceiptsResponse{}, err
	}

	body := buildReceiptsBody(receiptInfo.Receipts)

	err := s.sendEmail(receiptInfo.Email, "Your MrFood Receipts", body)
	if err != nil {
		return &pb.SendReceiptsResponse{}, models.ErrSendEmailFailed
	}

	return &pb.SendReceiptsResponse{}, nil
}

func (s *Service) Close() error {
	return s.redis.Close()
}

func (s *Service) checkRateLimit(ctx context.Context, email string) error {
	key := fmt.Sprintf("rate:email:%s", email)

	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return models.ErrRedisIncrFailed
	}
	if count == 1 {
		s.redis.Expire(ctx, key, s.rateLimit.RateLimitTTL)
	}
	if count > int64(s.rateLimit.EmailRateLimit) {
		return models.ErrRateLimitExceeded
	}
	return nil
}

func (s *Service) sendEmail(to, subject, body string) error {
	auth := smtp.PlainAuth("", s.smtp.User, s.smtp.Password, s.smtp.Host)
	addr := fmt.Sprintf("%s:%d", s.smtp.Host, s.smtp.Port)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		s.smtp.From, to, subject, body,
	)

	return smtp.SendMail(addr, auth, s.smtp.From, []string{to}, []byte(msg))
}

func buildReceiptsBody(receipts []*pb.Receipt) string {
	var sb strings.Builder

	sb.WriteString("<h2>Your Receipts</h2><table border='1' cellpadding='8'>")
	sb.WriteString("<tr><th>Description</th><th>Amount</th><th>Status</th><th>Type</th><th>Date</th></tr>")

	for _, r := range receipts {
		sb.WriteString(fmt.Sprintf(
			"<tr><td>%s</td><td>%.2f</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			r.PaymentDescription,
			r.Amount,
			r.CurrentPaymentStatus,
			r.PaymentType,
			r.CreatedAt.AsTime().Format("2006-01-02 15:04"),
		))
	}

	sb.WriteString("</table>")
	return sb.String()
}
