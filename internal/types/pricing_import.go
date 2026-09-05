package types

// PricingImportRule is one immutable pricing fact plus an optional instruction
// to close the preceding fact at this rule's effective_from. ClosesRuleID is an
// import instruction only and is never persisted on model_pricing.
type PricingImportRule struct {
	Pricing      ModelPricing
	ClosesRuleID *string
}

// PricingImportResult reports deterministic outcomes for a whole-file import.
type PricingImportResult struct {
	Inserted int
	NoOp     int
	Closed   int
}
