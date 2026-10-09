package classifier

import (
	"context"

	"github.com/sdskill/archintel/pkg/model"
)

// Classifier defines the standard contract for evaluating workload dimensions
// based on normalized requirements.
type Classifier interface {
	// Classify analyzes NormalizedRequirements and returns multi-label trait scores.
	Classify(ctx context.Context, req *model.NormalizedRequirements) (*model.ClassificationResult, error)
}
