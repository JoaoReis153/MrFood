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
	body, err := io.ReadAll(io.LimitReader(r.Body, 512*1024)) // 512KB limit
	if err != nil {
		slog.Error("webhook: failed to read body", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("Stripe-Signature")

	event, err := webhook.ConstructEvent(body, sig, h.webhookSecret)
	if err != nil {
		slog.Warn("webhook: signature verification failed", "error", err)
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	slog.Info("webhook: received event", "type", event.Type)

	switch event.Type {
	case "payment_intent.succeeded":
		h.handlePaymentSucceeded(w, r, event)
	default:
		slog.Debug("webhook: unhandled event type", "type", event.Type)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *Handler) handlePaymentSucceeded(w http.ResponseWriter, r *http.Request, event stripe.Event) {
	var pi stripe.PaymentIntent

	if err := json.Unmarshal(event.Data.Raw, &pi); err != nil {
		slog.Error("webhook: failed to parse payment_intent", "error", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	slog.Info("webhook: payment succeeded", "payment_intent_id", pi.ID)

	if err := h.confirmPayment(r.Context(), pi.ID); err != nil {
		slog.Error("webhook: confirmPayment failed", "payment_intent_id", pi.ID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
