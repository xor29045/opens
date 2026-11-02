package handlers

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"opensex/backend/internal/apperr"
	"opensex/backend/internal/dto"
)

func (h *Handler) Plans(c *gin.Context) {
	p, err := h.Svc.Billing.Plans(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"items": dto.MapSlice(p, dto.NewPlan)})
}

func (h *Handler) CurrentSubscription(c *gin.Context) {
	v, err := h.Svc.Billing.Current(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewSubscriptionResponse(v))
}

func (h *Handler) Subscribe(c *gin.Context) {
	var req dto.SubscribeRequest
	if !h.bind(c, &req) {
		return
	}
	p, err := h.Svc.Billing.Subscribe(c.Request.Context(), actor(c), req.PlanCode)
	if err != nil {
		fail(c, err)
		return
	}
	created(c, dto.NewPayment(*p))
}

func (h *Handler) Renew(c *gin.Context) {
	p, err := h.Svc.Billing.Renew(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	created(c, dto.NewPayment(*p))
}

func (h *Handler) CancelSubscription(c *gin.Context) {
	v, err := h.Svc.Billing.Cancel(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewSubscriptionResponse(v))
}

func (h *Handler) ResumeSubscription(c *gin.Context) {
	v, err := h.Svc.Billing.Resume(c.Request.Context(), actor(c))
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewSubscriptionResponse(v))
}

func (h *Handler) Payments(c *gin.Context) {
	p := page(c)
	rows, total, err := h.Svc.Billing.ListPayments(c.Request.Context(), actor(c), p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(rows, dto.NewPayment), p, total))
}

func (h *Handler) Payment(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	p, err := h.Svc.Billing.GetPayment(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.NewPayment(*p))
}

func (h *Handler) Invoices(c *gin.Context) {
	p := page(c)
	rows, total, err := h.Svc.Billing.Invoices(c.Request.Context(), actor(c), p)
	if err != nil {
		fail(c, err)
		return
	}
	ok(c, dto.Paginate(dto.MapSlice(rows, dto.NewInvoice), p, total))
}

func (h *Handler) InvoicePDF(c *gin.Context) {
	id, valid := uuidParam(c, "id")
	if !valid {
		return
	}
	inv, data, err := h.Svc.Billing.InvoicePDF(c.Request.Context(), actor(c), id)
	if err != nil {
		fail(c, err)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="facture-`+inv.Number+`.pdf"`)
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, "application/pdf", data)
}

const maxIPNBytes = 64 << 10

// NOWPaymentsWebhook receives IPNs: raw body, signature checked in the service.
func (h *Handler) NOWPaymentsWebhook(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxIPNBytes+1))
	if err != nil || len(body) > maxIPNBytes {
		fail(c, apperr.New(apperr.KindBadRequest, "payload_too_large", "IPN body too large"))
		return
	}
	if err := h.Svc.Billing.HandleIPN(c.Request.Context(), body, c.GetHeader("x-nowpayments-sig")); err != nil {
		fail(c, err)
		return
	}
	ok(c, gin.H{"status": "ok"})
}
