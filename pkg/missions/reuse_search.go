package missions

import (
	"sort"
	"strings"
	"unicode"
)

func findReusableMissionMatches(candidates []Mission, query string, limit int) []ReusableMissionMatch {
	terms := searchTerms(query)
	if len(terms) == 0 {
		return nil
	}

	matches := make([]ReusableMissionMatch, 0, len(candidates))
	for _, mission := range candidates {
		if !IsReusableMissionStatus(mission.Status) {
			continue
		}

		score, matchedTerms := scoreMissionMatch(mission, terms)
		if score == 0 {
			continue
		}

		matches = append(matches, ReusableMissionMatch{
			Mission:      mission,
			Score:        score,
			MatchedTerms: matchedTerms,
		})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Score == matches[j].Score {
			if matches[i].Mission.UpdatedAt.Equal(matches[j].Mission.UpdatedAt) {
				return matches[i].Mission.CreatedAt.After(matches[j].Mission.CreatedAt)
			}
			return matches[i].Mission.UpdatedAt.After(matches[j].Mission.UpdatedAt)
		}
		return matches[i].Score > matches[j].Score
	})

	if limit > 0 && len(matches) > limit {
		return matches[:limit]
	}
	return matches
}

func scoreMissionMatch(mission Mission, terms []string) (float64, []string) {
	weightedFields := []struct {
		text   string
		weight float64
	}{
		{text: strings.ToLower(mission.Title), weight: 5},
		{text: strings.ToLower(mission.Goal), weight: 4},
		{text: strings.ToLower(mission.Scope), weight: 3},
		{text: strings.ToLower(mission.Charter), weight: 2},
	}

	matched := make([]string, 0, len(terms))
	matchedSet := make(map[string]struct{}, len(terms))
	var score float64
	for _, term := range terms {
		for _, field := range weightedFields {
			if field.text != "" && strings.Contains(field.text, term) {
				score += field.weight
				if _, exists := matchedSet[term]; !exists {
					matchedSet[term] = struct{}{}
					matched = append(matched, term)
				}
			}
		}
	}
	if score == 0 {
		return 0, nil
	}

	return score / float64(len(terms)), matched
}

func searchTerms(query string) []string {
	parts := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	seen := make(map[string]struct{}, len(parts))
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) < 3 {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		terms = append(terms, part)
	}
	if len(terms) == 0 {
		normalized := strings.TrimSpace(strings.ToLower(query))
		if normalized != "" {
			terms = append(terms, normalized)
		}
	}
	return terms
}
