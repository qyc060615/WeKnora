package interfaces

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

type PricingRepository interface {
	CreatePricing(ctx context.Context, rule *types.ModelPricing) error
	ResolvePricing(ctx context.Context, provider, modelName string, callType types.CallType, at time.Time) (*types.ModelPricing, error)
	CreateCost(ctx context.Context, cost *types.ModelUsageCost) error
	GetCostByUsageID(ctx context.Context, usageID string) (*types.ModelUsageCost, error)
}
