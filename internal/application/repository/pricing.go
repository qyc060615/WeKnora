package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type pricingRepository struct{ db *gorm.DB }

func NewPricingRepository(db *gorm.DB) interfaces.PricingRepository {
	return &pricingRepository{db: db}
}

func (r *pricingRepository) CreatePricing(ctx context.Context, rule *types.ModelPricing) error {
	if rule == nil {
		return fmt.Errorf("model_pricing: rule is nil")
	}
	if err := rule.Validate(); err != nil {
		return err
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		q := tx.Model(&types.ModelPricing{}).
			Where("resolved_provider = ? AND resolved_model_name = ? AND call_type = ?", rule.ResolvedProvider, rule.ResolvedModelName, rule.CallType).
			Where("effective_to IS NULL OR effective_to > ?", rule.EffectiveFrom)
		if rule.EffectiveTo != nil {
			q = q.Where("effective_from < ?", *rule.EffectiveTo)
		}
		if err := q.Count(&count).Error; err != nil {
			return fmt.Errorf("check model_pricing overlap: %w", err)
		}
		if count != 0 {
			return fmt.Errorf("model_pricing: overlapping effective interval")
		}
		return tx.Create(rule).Error
	})
}

func (r *pricingRepository) ResolvePricing(ctx context.Context, provider, modelName string, callType types.CallType, at time.Time) (*types.ModelPricing, error) {
	if provider == "" || modelName == "" || at.IsZero() {
		return nil, nil
	}
	var rules []types.ModelPricing
	err := r.db.WithContext(ctx).
		Where("resolved_provider = ? AND resolved_model_name = ? AND call_type = ?", provider, modelName, callType).
		Where("effective_from <= ?", at).
		Where("effective_to IS NULL OR effective_to > ?", at).
		Order("effective_from DESC").Limit(2).Find(&rules).Error
	if err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, nil
	}
	if len(rules) > 1 {
		return nil, fmt.Errorf("model_pricing: overlapping rules found during resolution")
	}
	return &rules[0], nil
}

func (r *pricingRepository) CreateCost(ctx context.Context, cost *types.ModelUsageCost) error {
	if cost == nil || cost.UsageID == "" {
		return fmt.Errorf("model_usage_cost: usage_id is required")
	}
	return r.db.WithContext(ctx).Create(cost).Error
}

func (r *pricingRepository) GetCostByUsageID(ctx context.Context, usageID string) (*types.ModelUsageCost, error) {
	var cost types.ModelUsageCost
	err := r.db.WithContext(ctx).Where("usage_id = ?", usageID).First(&cost).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cost, nil
}
