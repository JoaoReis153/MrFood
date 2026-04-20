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

type RedisClient interface {
	Incr(ctx context.Context, key string) *redis.IntCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
	Close() error
}

type Mailer interface {
	Send(to, subject, body string) error
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
}

type SMTPMailer struct {
	config   SMTPConfig
	sendMail func(addr string, a smtp.Auth, from string, to []string, msg []byte) error
}

type Service struct {
	redis     RedisClient
	mailer    Mailer
	rateLimit RateLimitConfig
}

type RateLimitConfig struct {
	EmailRateLimit int
	RateLimitTTL   time.Duration
}

func New(redis RedisClient, smtpCfg SMTPConfig, rateLimit RateLimitConfig) *Service {
	if redis == nil {
		panic("nil redis client")
	}
	return &Service{redis: redis, mailer: NewSMTPMailer(smtpCfg), rateLimit: rateLimit}
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	return &SMTPMailer{config: cfg, sendMail: smtp.SendMail}
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

	err := s.mailer.Send(
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

	if receiptInfo.UserEmail == "" {
		return &pb.SendReceiptsResponse{}, models.ErrInvalidEmail
	}
	if len(receiptInfo.Receipts) == 0 {
		return &pb.SendReceiptsResponse{}, models.ErrEmptyReceipts
	}

	if err := s.checkRateLimit(ctx, receiptInfo.UserEmail); err != nil {
		return &pb.SendReceiptsResponse{}, err
	}

	body := buildReceiptsBody(receiptInfo.Receipts)

	err := s.mailer.Send(receiptInfo.UserEmail, "Your MrFood Receipts", body)
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

func (m *SMTPMailer) Send(to, subject, body string) error {
	auth := smtp.PlainAuth("", m.config.User, m.config.Password, m.config.Host)
	addr := fmt.Sprintf("%s:%d", m.config.Host, m.config.Port)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.config.From, to, subject, body,
	)
	return m.sendMail(addr, auth, m.config.From, []string{to}, []byte(msg))
}

func buildReceiptsBody(receipts []*pb.Receipt) string {
	var sb strings.Builder

	sb.WriteString("<h2>Your Receipts</h2><table border='1' cellpadding='8'>")
	sb.WriteString("<tr><th>Description</th><th>Amount</th><th>Status</th><th>Type</th><th>Date</th></tr>")

	for _, r := range receipts {
		fmt.Fprintf(&sb,
			"<tr><td>%s</td><td>%.2f</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			r.PaymentDescription,
			r.Amount,
			r.CurrentPaymentStatus,
			r.PaymentType,
			r.CreatedAt.AsTime().Format("2006-01-02 15:04"),
		)
	}

	sb.WriteString("</table>")
	return sb.String()
}
