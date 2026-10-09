package knowledge

import (
	"testing"

	"github.com/sdskill/archintel/pkg/model"
)

func TestPatternCatalogRetrieval(t *testing.T) {
	cat := NewCatalog()

	pat, found := cat.Get("PAT-MODULAR-MONOLITH")
	if !found {
		t.Fatalf("expected PAT-MODULAR-MONOLITH to be found in catalog")
	}
	if pat.Category != CategoryTopology {
		t.Errorf("expected CategoryTopology, got %v", pat.Category)
	}

	dataPatterns := cat.FindByCategory(CategoryData)
	if len(dataPatterns) == 0 {
		t.Errorf("expected data patterns to be returned")
	}

	messagingPatterns := cat.FindByCategory(CategoryMessaging)
	if len(messagingPatterns) < 2 {
		t.Errorf("expected at least 2 messaging patterns (outbox and skip-locked queue), got %d", len(messagingPatterns))
	}
}

func TestPatternCatalogTraitMatching(t *testing.T) {
	cat := NewCatalog()

	traits := []model.WorkloadTrait{
		model.TraitTransactionHeavy,
		model.TraitHighContention,
	}

	matched := cat.MatchTraits(traits)
	if len(matched) == 0 {
		t.Fatalf("expected matched patterns for transaction and high contention")
	}

	foundPessimistic := false
	for _, p := range matched {
		if p.ID == "PAT-PESSIMISTIC-LOCKING" {
			foundPessimistic = true
		}
	}
	if !foundPessimistic {
		t.Errorf("expected PAT-PESSIMISTIC-LOCKING to match TraitHighContention")
	}
}
