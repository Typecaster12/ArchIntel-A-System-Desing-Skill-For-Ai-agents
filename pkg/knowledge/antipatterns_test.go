package knowledge

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestAntiPatternCatalogRetrieval(t *testing.T) {
	cat := NewAntiPatternCatalog()

	ap, found := cat.Get("ANTI-SYNC-IN-TX")
	if !found {
		t.Fatalf("expected ANTI-SYNC-IN-TX to be in catalog")
	}
	if ap.Risk != RiskCritical {
		t.Errorf("expected RiskCritical, got %v", ap.Risk)
	}

	all := cat.All()
	if len(all) < 5 {
		t.Errorf("expected at least 5 anti-patterns, got %d", len(all))
	}
}

func TestAntiPatternRelevance(t *testing.T) {
	cat := NewAntiPatternCatalog()

	req := &model.NormalizedRequirements{
		ProjectName: "CheckoutApp",
		Domain:      model.DomainECommerce,
		Traffic: model.TrafficProfile{
			ExpectedRPS: 50, // Moderate RPS (< 500)
		},
		Data: model.DataProfile{
			RequiresACID: true,
		},
	}

	relevant := cat.RelevantAntiPatterns(req)
	if len(relevant) == 0 {
		t.Fatalf("expected relevant anti-patterns for ecommerce")
	}

	foundSyncTx := false
	foundPrematureMicro := false
	for _, ap := range relevant {
		if ap.ID == "ANTI-SYNC-IN-TX" {
			foundSyncTx = true
		}
		if ap.ID == "ANTI-PREMATURE-MICROSERVICES" {
			foundPrematureMicro = true
		}
	}

	if !foundSyncTx {
		t.Errorf("expected ANTI-SYNC-IN-TX to be flagged for ACID ecommerce")
	}
	if !foundPrematureMicro {
		t.Errorf("expected ANTI-PREMATURE-MICROSERVICES to be flagged for low-RPS app")
	}
}
