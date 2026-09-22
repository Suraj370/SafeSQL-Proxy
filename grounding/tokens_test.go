package grounding

import (
	"strings"
	"testing"

	semantic "github.com/liliang-cn/semantic-go"
)

var testModel = semantic.Model{
	Metrics: []semantic.Metric{
		{Name: "revenue", Synonyms: []string{"top line", "sales figures"}, Description: "total sales amount"},
		{Name: "units_sold", Synonyms: []string{"volume", "pieces moved"}, Description: "units sold"},
	},
}

func TestTokensOfASCIIUnchanged(t *testing.T) {
	toks := tokensOf("Revenue by Region")
	if !toks["revenue"] || !toks["region"] {
		t.Fatalf("ascii tokens missing: %v", toks)
	}
	if toks["by"] {
		t.Fatalf("stopword leaked into tokens: %v", toks)
	}
}

func TestFtsQueryKeepsWords(t *testing.T) {
	// A question must not collapse to an empty/degenerate query.
	q := ftsQuery("revenue by store region")
	if !strings.Contains(q, "revenue") {
		t.Fatalf("ftsQuery dropped a token, got %q", q)
	}
}

func TestMemLexicalSurfacesBySynonym(t *testing.T) {
	// A metric whose synonym matches the question must score, with no embedder
	// and regardless of the FTS engine's tokenizer.
	g := &Grounder{model: &testModel}
	scores := g.memLexical("what's the top line by store region")
	if scores["revenue"] <= 0 {
		t.Fatalf("revenue not surfaced by synonym; scores=%v", scores)
	}
}
