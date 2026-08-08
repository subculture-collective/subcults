package touring

import "strings"

// ReconciliationCandidate is a normalized import claim used only to decide
// whether an operator must review a potential duplicate.
type ReconciliationCandidate struct {
	Provider   string
	ExternalID string
	ActName    string
	LocalDate  string
	PlaceName  string
	VenueName  string
	EventKind  string
}

type ReconciliationResult struct {
	AutoMerged       int
	ReviewCandidates int
}

// Reconcile auto-merges only a repeated provider external ID. Similar
// festival, set, and afterparty claims always go to review.
func Reconcile(candidates []ReconciliationCandidate) ReconciliationResult {
	result := ReconciliationResult{}
	strong := make(map[string]struct{})
	weak := make(map[string]ReconciliationCandidate)
	for _, candidate := range candidates {
		strongKey := normalize(candidate.Provider) + "\x00" + normalize(candidate.ExternalID)
		if candidate.Provider != "" && candidate.ExternalID != "" {
			if _, exists := strong[strongKey]; exists {
				result.AutoMerged++
				continue
			}
			strong[strongKey] = struct{}{}
		}
		weakKey := strings.Join([]string{
			normalize(candidate.ActName), candidate.LocalDate, normalize(candidate.PlaceName), normalize(candidate.VenueName),
		}, "\x00")
		if previous, exists := weak[weakKey]; exists &&
			(previous.EventKind != candidate.EventKind || previous.ExternalID != candidate.ExternalID || previous.Provider != candidate.Provider) {
			result.ReviewCandidates++
			continue
		}
		weak[weakKey] = candidate
	}
	return result
}

func normalize(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}
