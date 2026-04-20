package policy

import (
	"regexp"
	"strings"

	"github.com/dmvianna/chatbot-prototype/internal/types"
)

var splitSentences = regexp.MustCompile(`[.!?]\s+`)
var namedEntity = regexp.MustCompile(`\b[A-Z][a-z]+(?:\s+[A-Z][a-z]+)*\b`)

type Validator struct {
	minCoverage float64
}

func NewValidator(minCoverage float64) *Validator {
	return &Validator{minCoverage: minCoverage}
}

func (v *Validator) Validate(answer string, citations []string, evidence []types.RetrievalResult) types.ValidationReport {
	validCitations := citationSet(citations, evidence)
	evidenceText := strings.ToLower(evidenceConcat(evidence, validCitations))

	sentences := splitIntoClaims(answer)
	if len(sentences) == 0 {
		return types.ValidationReport{Valid: false}
	}
	var supported int
	var unsupportedClaims []string
	for _, sentence := range sentences {
		if claimSupported(sentence, evidenceText) {
			supported++
			continue
		}
		unsupportedClaims = append(unsupportedClaims, sentence)
	}
	coverage := float64(supported) / float64(len(sentences))
	unsupportedEntities := entityMismatches(answer, evidenceText)
	valid := coverage >= v.minCoverage && len(unsupportedClaims) == 0 && len(unsupportedEntities) == 0 && len(validCitations) > 0

	return types.ValidationReport{
		Coverage:            coverage,
		UnsupportedClaims:   unsupportedClaims,
		UnsupportedEntities: unsupportedEntities,
		Valid:               valid,
	}
}

func splitIntoClaims(answer string) []string {
	parts := splitSentences.Split(strings.TrimSpace(answer), -1)
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) < 12 {
			continue
		}
		out = append(out, p)
	}
	return out
}

func claimSupported(claim, evidence string) bool {
	tokens := tokenise(claim)
	if len(tokens) == 0 {
		return false
	}
	var matches int
	for _, t := range tokens {
		if len(t) <= 3 {
			continue
		}
		if strings.Contains(evidence, t) {
			matches++
		}
	}
	if matches == 0 {
		return false
	}
	ratio := float64(matches) / float64(len(tokens))
	return ratio >= 0.30
}

func tokenise(s string) []string {
	s = strings.ToLower(s)
	s = strings.NewReplacer(",", " ", ".", " ", ";", " ", ":", " ", "\n", " ", "\t", " ", "(", " ", ")", " ").Replace(s)
	fields := strings.Fields(s)
	return fields
}

func entityMismatches(answer, evidence string) []string {
	matches := namedEntity.FindAllString(answer, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var unsupported []string
	for _, m := range matches {
		k := strings.ToLower(strings.TrimSpace(m))
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		if !strings.Contains(evidence, k) {
			unsupported = append(unsupported, m)
		}
	}
	return unsupported
}

func citationSet(citations []string, evidence []types.RetrievalResult) map[string]struct{} {
	evidenceIDs := map[string]struct{}{}
	for _, e := range evidence {
		evidenceIDs[e.Chunk.ID] = struct{}{}
	}
	out := map[string]struct{}{}
	for _, c := range citations {
		if _, ok := evidenceIDs[c]; ok {
			out[c] = struct{}{}
		}
	}
	return out
}

func evidenceConcat(evidence []types.RetrievalResult, citations map[string]struct{}) string {
	var sb strings.Builder
	for _, e := range evidence {
		if len(citations) > 0 {
			if _, ok := citations[e.Chunk.ID]; !ok {
				continue
			}
		}
		sb.WriteString(e.Chunk.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}
