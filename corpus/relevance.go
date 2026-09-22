package corpus

import "strings"

// Retrieval always returns something, and something is not always a citation.
//
// Asked "forklift maintenance intervals" against a corpus whose
// only document explains why revenue is net of refunds, the store returned the
// revenue memo. It was not wrong to: top-k over one document has one answer,
// and the fusion score that orders results is a rank, not a similarity, so
// there is no threshold on it that means "related".
//
// A brief that prints an unrelated passage under the heading "written about it"
// is worse than one that prints nothing, because the reader will assume the
// connection is real — that is the entire reason the passage is there. So
// retrieval needs a floor, and the floor has to be something a reader would
// accept as evidence rather than a tuned number.
//
// The floor used here is shared vocabulary: a passage is a citation only if it
// repeats a term from the question. That is deliberately crude. It will drop a
// passage that answers the question in different words, which is a real loss
// and a visible one — the brief says nothing rather than something wrong. It
// will not invent a connection, which is the failure that costs more.
//
// Single-letter words are excluded from counting as a shared term: they appear
// in nearly every document ever written, and sharing one is not evidence of
// anything.

// bearsAny reports whether a passage repeats any term from any of the sources
// the caller is willing to match on — the question, and the vocabulary of
// whatever metric the question resolved to.
//
// Both matter. Filtering on the question alone would discard the passages that
// the metric's own synonyms found, which is the silent way to make query
// enrichment do nothing.
func bearsAny(sources []string, passage string) bool {
	want := map[string]bool{}
	for _, src := range sources {
		for t := range terms(src) {
			want[t] = true
		}
	}
	if len(want) == 0 {
		return true // nothing to match on; let the ranker decide
	}
	have := terms(passage)
	for t := range want {
		if have[t] {
			return true
		}
	}
	return false
}

// terms is the set of things worth matching on: words of two or more letters.
func terms(s string) map[string]bool {
	out := map[string]bool{}
	for _, t := range tokens(s) {
		if len([]rune(t)) >= 2 {
			out[t] = true
		}
	}
	return out
}

// trimmed is strings.TrimSpace, named so the filter below reads as one thought.
func trimmed(s string) string { return strings.TrimSpace(s) }
