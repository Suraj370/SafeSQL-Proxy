package ingest

import (
	"testing"

	"github.com/suraj370/safesqlproxy/connectors"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"café", "café"},
		{"reçu", "reçu"},
		{"Order ID", "order_id"},
		{"Montant (Total)", "montant_total"},
		{"  Total  ", "total"},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestInferMapping_NonASCIIZeroConfig(t *testing.T) {
	schema := connectors.SourceSchema{
		Name: "ventes_boutique",
		Fields: []connectors.Field{
			{Name: "boutique", Type: "text"},
			{Name: "catégorie", Type: "text"},
			{Name: "montant", Type: "numeric"},
		},
	}
	plan := InferMapping(schema, "some_table", nil)
	if len(plan.Fields) != len(schema.Fields) {
		t.Fatalf("got %d field maps, want %d", len(plan.Fields), len(schema.Fields))
	}
	seen := map[string]bool{}
	for _, fm := range plan.Fields {
		if fm.Target == "" {
			t.Errorf("field %q normalized to empty target", fm.Source)
		}
		if fm.Target != fm.Source {
			t.Errorf("field %q normalized to %q, want the name unchanged", fm.Source, fm.Target)
		}
		if seen[fm.Target] {
			t.Errorf("duplicate target %q", fm.Target)
		}
		seen[fm.Target] = true
	}
}
