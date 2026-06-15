package certificate

import (
	"github.com/ing-bank/golibs/pkg/slices"
)

// MatchAnyCommonName checks if any of the provided common names (cns) exist in the Certificate's CNs.
// It returns the first matching common name and true if a match is found, otherwise an empty string and false.
func (h Certificate) MatchAnyCommonName(cns []string) (string, bool) {
	return slices.MatchAny(h.CNs, cns)
}
