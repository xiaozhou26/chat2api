package service

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type UsageResponse struct {
	Object          string        `json:"object"`
	TotalUsage      int           `json:"total_usage"`
	CurrentUsageUSD float64       `json:"current_usage_usd"`
	DailyCosts      []interface{} `json:"daily_costs"`
}

type TokenUsageResponse struct {
	Used            int   `json:"used"`
	Total           int   `json:"total"`
	Remaining       int   `json:"remaining"`
	Usage           int   `json:"usage"`
	Limit           int   `json:"limit"`
	UsedQuota       int   `json:"used_quota"`
	RemainQuota     int   `json:"remain_quota"`
	Quota           int   `json:"quota"`
	TotalQuota      int   `json:"total_quota"`
	UsedTokens      int   `json:"used_tokens"`
	TotalTokens     int   `json:"total_tokens"`
	RemainingTokens int   `json:"remaining_tokens"`
	UnlimitedQuota  bool  `json:"unlimited_quota"`
	ExpiresAt       int64 `json:"expires_at"`
	AccessUntil     int64 `json:"access_until"`
}

type BillingSubscriptionResponse struct {
	Object                string  `json:"object"`
	BillingPeriod         string  `json:"billing_period"`
	CurrentPeriodStart    int64   `json:"current_period_start"`
	CurrentPeriodEnd      int64   `json:"current_period_end"`
	Plan                  string  `json:"plan"`
	AccountName           string  `json:"account_name"`
	SoftLimitUSD          float64 `json:"soft_limit_usd"`
	HardLimitUSD          float64 `json:"hard_limit_usd"`
	SystemHardLimitUSD    float64 `json:"system_hard_limit_usd"`
	SoftLimit             float64 `json:"soft_limit"`
	HardLimit             float64 `json:"hard_limit"`
	SystemHardLimit       float64 `json:"system_hard_limit"`
	HasPaymentMethod      bool    `json:"has_payment_method"`
	AccessUntil           int64   `json:"access_until"`
	HasActiveSubscription bool    `json:"has_active_subscription"`
}

func Usage(c *gin.Context) {
	c.JSON(http.StatusOK, UsageResponse{
		Object:          "list",
		TotalUsage:      0,
		CurrentUsageUSD: 0,
		DailyCosts:      []interface{}{},
	})
}

func TokenUsage(c *gin.Context) {
	total := 100
	used := 0
	remaining := total - used
	accessUntil := time.Now().AddDate(100, 0, 0).Unix()

	c.JSON(http.StatusOK, TokenUsageResponse{
		Used:            used,
		Total:           total,
		Remaining:       remaining,
		Usage:           used,
		Limit:           total,
		UsedQuota:       used,
		RemainQuota:     remaining,
		Quota:           total,
		TotalQuota:      total,
		UsedTokens:      used,
		TotalTokens:     total,
		RemainingTokens: remaining,
		UnlimitedQuota:  false,
		ExpiresAt:       accessUntil,
		AccessUntil:     accessUntil,
	})
}

func BillingSubscription(c *gin.Context) {
	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, 0)
	accessUntil := now.AddDate(100, 0, 0).Unix()

	c.JSON(http.StatusOK, BillingSubscriptionResponse{
		Object:                "billing_subscription",
		BillingPeriod:         "monthly",
		CurrentPeriodStart:    periodStart.Unix(),
		CurrentPeriodEnd:      periodEnd.Unix(),
		Plan:                  "ChatGPT Pro",
		AccountName:           "ChatGPT Pro",
		SoftLimitUSD:          100,
		HardLimitUSD:          100,
		SystemHardLimitUSD:    100,
		SoftLimit:             100,
		HardLimit:             100,
		SystemHardLimit:       100,
		HasPaymentMethod:      true,
		AccessUntil:           accessUntil,
		HasActiveSubscription: true,
	})
}
