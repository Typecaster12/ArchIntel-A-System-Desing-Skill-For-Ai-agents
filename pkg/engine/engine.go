package engine

import (
	"context"
	"fmt"

	"github.com/sdskill/archintel/pkg/classifier"
	"github.com/sdskill/archintel/pkg/contract"
	"github.com/sdskill/archintel/pkg/evaluator"
	"github.com/sdskill/archintel/pkg/knowledge"
	"github.com/sdskill/archintel/pkg/model"
	"github.com/sdskill/archintel/pkg/normalizer"
	"github.com/sdskill/archintel/pkg/validator"
)

// EngineOptions configures the Engine components.
type EngineOptions struct {
	Classifier         classifier.Classifier
	PatternCatalog     *knowledge.Catalog
	AntiPatternCatalog *knowledge.AntiPatternCatalog
}

// Engine coordinates the entire 7-stage architectural decision pipeline.
type Engine struct {
	normalizer   *normalizer.Normalizer
	gapDetector  *normalizer.GapDetector
	classifier   classifier.Classifier
	catalog      *knowledge.Catalog
	antiPatterns *knowledge.AntiPatternCatalog
	evaluator    *evaluator.Evaluator
	generator    *contract.Generator
	validator    *validator.Validator
}

// NewEngine creates an Engine instance with supplied or default dependencies.
func NewEngine(opts EngineOptions) *Engine {
	cls := opts.Classifier
	if cls == nil {
		cls = classifier.NewRuleBasedClassifier()
	}

	cat := opts.PatternCatalog
	if cat == nil {
		cat = knowledge.NewCatalog()
	}

	ap := opts.AntiPatternCatalog
	if ap == nil {
		ap = knowledge.NewAntiPatternCatalog()
	}

	return &Engine{
		normalizer:   normalizer.New(),
		gapDetector:  normalizer.NewGapDetector(),
		classifier:   cls,
		catalog:      cat,
		antiPatterns: ap,
		evaluator:    evaluator.New(cat),
		generator:    contract.New(),
		validator:    validator.New(),
	}
}

// AnalysisResponse holds requirement normalization and gap analysis output.
type AnalysisResponse struct {
	Normalized *model.NormalizedRequirements `json:"normalized"`
	Gaps       *normalizer.GapAnalysis       `json:"gaps"`
}

// RecommendationResponse holds the full architectural recommendation and contract.
type RecommendationResponse struct {
	Normalized           *model.NormalizedRequirements `json:"normalized"`
	Contract             *model.ArchitectureContract   `json:"contract"`
	Classification       *model.ClassificationResult   `json:"classification"`
	Gaps                 *normalizer.GapAnalysis       `json:"gaps"`
	DecisionSummary      *evaluator.DecisionSummary    `json:"decision_summary"`
	RelevantAntiPatterns []knowledge.AntiPattern       `json:"relevant_antipatterns"`
	ExecutiveSummary     string                        `json:"executive_summary"`
}

// TradeoffResponse holds multi-criteria candidate rankings and rejection rationales.
type TradeoffResponse struct {
	DecisionSummary      *evaluator.DecisionSummary  `json:"decision_summary"`
	RejectedAlternatives []model.RejectedAlternative `json:"rejected_alternatives"`
}

// AnalyzeRequirements parses raw requirements, normalizes metrics, and flags material gaps.
func (e *Engine) AnalyzeRequirements(raw model.RawRequirement) (*AnalysisResponse, error) {
	norm := e.normalizer.Normalize(raw)
	gaps := e.gapDetector.Analyze(norm)

	return &AnalysisResponse{
		Normalized: norm,
		Gaps:       gaps,
	}, nil
}

// ClassifyWorkload executes Stage 1 through Stage 3 to evaluate workload traits.
func (e *Engine) ClassifyWorkload(ctx context.Context, raw model.RawRequirement) (*model.ClassificationResult, error) {
	norm := e.normalizer.Normalize(raw)
	return e.classifier.Classify(ctx, norm)
}

// RecommendArchitecture executes the full 7-stage pipeline and produces the ArchitectureContract.
func (e *Engine) RecommendArchitecture(ctx context.Context, raw model.RawRequirement) (*RecommendationResponse, error) {
	// Stage 1: Normalize requirements
	norm := e.normalizer.Normalize(raw)

	// Stage 2: Identify gaps and missing information
	gaps := e.gapDetector.Analyze(norm)

	// Stage 3: Classify workload
	class, err := e.classifier.Classify(ctx, norm)
	if err != nil {
		return nil, fmt.Errorf("workload classification failed: %w", err)
	}

	// Stage 4 & 5: Evaluate alternatives and trade-offs
	summary := e.evaluator.Evaluate(norm, class)

	// Stage 6: Generate ArchitectureContract
	archContract, err := e.generator.Generate(norm, class, summary)
	if err != nil {
		return nil, fmt.Errorf("contract generation failed: %w", err)
	}

	// Retrieve pertinent anti-patterns to guard against
	antiPats := e.antiPatterns.RelevantAntiPatterns(norm)

	execSummary := fmt.Sprintf(
		"Recommended %s architecture for %s. Consistency: %s, Concurrency Locking: %s, Queue: %s.",
		summary.RecommendedTopology,
		norm.ProjectName,
		summary.RecommendedDataRules.IsolationLevel,
		summary.RecommendedDataRules.Locking,
		summary.RecommendedQueueRules.QueueMechanism,
	)

	return &RecommendationResponse{
		Normalized:           norm,
		Contract:             archContract,
		Classification:       class,
		Gaps:                 gaps,
		DecisionSummary:      summary,
		RelevantAntiPatterns: antiPats,
		ExecutiveSummary:     execSummary,
	}, nil
}

// EvaluateTradeoffs focuses specifically on candidate evaluations and rejection rationales.
func (e *Engine) EvaluateTradeoffs(ctx context.Context, raw model.RawRequirement) (*TradeoffResponse, error) {
	norm := e.normalizer.Normalize(raw)
	class, err := e.classifier.Classify(ctx, norm)
	if err != nil {
		return nil, err
	}

	summary := e.evaluator.Evaluate(norm, class)
	rejectionGen := evaluator.NewRejectionGenerator()
	rejections := rejectionGen.GenerateRejections(norm, class, summary)

	return &TradeoffResponse{
		DecisionSummary:      summary,
		RejectedAlternatives: rejections,
	}, nil
}

// ValidateArchitecture checks proposed implementation files or declarations against an ArchitectureContract.
func (e *Engine) ValidateArchitecture(contract *model.ArchitectureContract, proposal validator.ImplementationProposal) *validator.ValidationReport {
	return e.validator.Validate(contract, proposal)
}
