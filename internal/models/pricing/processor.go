package pricing

import (
	"context"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type Processor struct {
	repo       interfaces.PricingRepository
	calculator *Calculator
}

func NewProcessor(repo interfaces.PricingRepository) *Processor {
	return &Processor{repo: repo, calculator: NewCalculator()}
}

func (p *Processor) Process(ctx context.Context, usage *types.ModelUsage) error {
	var rule *types.ModelPricing
	var err error
	if usage.ResolvedProvider != "" && usage.ResolvedModelName != nil && usage.StartedAt != nil {
		rule, err = p.repo.ResolvePricing(ctx, usage.ResolvedProvider, *usage.ResolvedModelName, usage.CallType, *usage.StartedAt)
		if err != nil {
			return err
		}
	}
	cost, err := p.calculator.Calculate(usage, rule)
	if err != nil {
		return err
	}
	if cost.CalculatedAt.IsZero() {
		cost.CalculatedAt = time.Now().UTC()
	}
	return p.repo.CreateCost(ctx, cost)
}
