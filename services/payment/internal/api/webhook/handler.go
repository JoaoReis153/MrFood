package webhook

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

type confirmPaymentFunc func(ctx context.Context, paymentIntentID string) error

type Handler struct {
	webhookSecret  string
	confirmPayment confirmPaymentFunc
}

func New(webhookSecret string, confirmPayment confirmPaymentFunc) *Handler {
	return &Handler{
		webhookSecret:  webhookSecret,
		confirmPayment: confirmPayment,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024)) // 512KB limit
	if err != nil {
		slog.ErrorContext(ctx, "failed to read webhook body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")

	event, err := webhook.ConstructEvent(body, sig, h.webhookSecret)
	if err != nil {
		slog.WarnContext(ctx, "webhook signature verification failed", "error", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	slog.InfoContext(ctx, "webhook event received", "type", event.Type)

	switch event.Type {
	case "payment_intent.succeeded":
		h.handlePaymentSucceeded(w, r, event)
	default:
		slog.DebugContext(ctx, "unhandled webhook event type", "type", event.Type)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) handlePaymentSucceeded(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	ctx := r.Context()

	var pi stripe.PaymentIntent

	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		slog.ErrorContext(ctx, "failed to parse payment_intent payload", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	slog.InfoContext(ctx, "payment succeeded", "payment_intent_id", pi.ID)

	if err := h.confirmPayment(ctx, pi.ID); err != nil {
		slog.ErrorContext(ctx, "failed to confirm payment", "payment_intent_id", pi.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
