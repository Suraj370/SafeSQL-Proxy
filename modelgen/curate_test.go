package modelgen

import (
	"strings"
	"testing"

	semantic "github.com/liliang-cn/semantic-go"
)

// `has` lives in generate_test.go.

func TestSynonymsComeFromTheAttribute(t *testing.T) {
	// store_region is asked about as "area", not as "store area" — the entity
	// prefix is there to make the name unique, not because anyone says it out
	// loud.
	for _, tc := range []struct {
		name string
		want string
	}{
		{"store_region", "area"},
		{"customer_city", "town"},
		{"product_category", "class"},
		{"staff_position", "role"},
	} {
		if got := deriveSynonyms(tc.name); !has(got, tc.want) {
			t.Errorf("deriveSynonyms(%q) = %v, missing %q", tc.name, got, tc.want)
		}
	}
}

func TestSynonymsComposeCountsAndRates(t *testing.T) {
	if got := deriveSynonyms("order_count"); !has(got, "purchase order tally") {
		t.Errorf("order_count = %v, wanted purchase order tally", got)
	}
	if got := deriveSynonyms("customer_count"); !has(got, "client tally") {
		t.Errorf("customer_count = %v, wanted client tally", got)
	}
	// A sum of money reads bare: "total value", not "total value total".
	if got := deriveSynonyms("order_amount_sum"); !has(got, "total value") {
		t.Errorf("order_amount_sum = %v, wanted total value", got)
	}
	// A whole-name entry wins over composing its parts.
	if got := deriveSynonyms("revenue"); !has(got, "top line") {
		t.Errorf("revenue = %v, wanted top line", got)
	}
}

func TestSynonymsStaySilentWhenUnsure(t *testing.T) {
	// Better an empty list than a made-up phrase nobody would ever type.
	for _, name := range []string{"widget_frobnicator", "xyz_abc", "foo"} {
		if got := deriveSynonyms(name); len(got) != 0 {
			t.Errorf("deriveSynonyms(%q) = %v, an unrecognized word should stay silent", name, got)
		}
	}
}

func TestPerUnitMetricsDoNotStealTheirDenominatorsWord(t *testing.T) {
	// order_amount_per_qty used to come back "quantity" — matched on the
	// denominator — and then competed with order_qty_sum for that word. Two
	// metrics answering to "quantity" means one of them silently returns the
	// wrong number.
	for _, name := range []string{"order_amount_per_qty", "sale_revenue_per_order", "revenue_per_head"} {
		if got := deriveSynonyms(name); len(got) != 0 {
			t.Errorf("deriveSynonyms(%q) = %v, a per-unit rate should not borrow its denominator's word", name, got)
		}
	}
	// The denominator itself keeps its word.
	if got := deriveSynonyms("order_qty_sum"); !has(got, "quantity") {
		t.Errorf("order_qty_sum = %v, wanted quantity", got)
	}
}

func TestPIIMasksContactColumnsButNotStoreNames(t *testing.T) {
	for _, col := range []string{"phone", "member_phone", "guest_mobile", "email", "id_card", "bank_card_no", "address", "tel"} {
		if !piiColumn(col) {
			t.Errorf("%q should be recognized as PII", col)
		}
	}
	// Masking every name column would break grouping by store — the single most
	// common thing anyone asks for — to protect a column a reviewer masks in one
	// line. And `hotel` must not trip the bare-`tel` rule.
	for _, col := range []string{"customer_name", "store_name", "hotel", "hotel_name", "telecom_provider", "category"} {
		if piiColumn(col) {
			t.Errorf("%q should not be treated as PII", col)
		}
	}
}

func TestMoneyGatesRatiosBuiltOnMoneyButNotOperationalRates(t *testing.T) {
	// Every reviewed model gates margin_rate and net_margin, and leaves
	// yield_rate / defect_rate / on_time_rate open. The test is the measure,
	// not the `_rate` suffix.
	for _, m := range []string{"revenue", "sale_gross_margin", "margin_rate", "net_profit", "rd_spend", "avg_ticket", "adr", "revpar", "order_amount_sum"} {
		if !moneyMetric(m, "") {
			t.Errorf("%q is money, should be gated", m)
		}
	}
	for _, m := range []string{"yield_rate", "defect_rate", "on_time_rate", "units_sold", "stock_on_hand", "occupancy", "produced_qty", "items_per_order"} {
		if moneyMetric(m, "") {
			t.Errorf("%q is not money, should not be gated", m)
		}
	}
	// The expression counts too: a metric named `total` over an `amount` column
	// is still money.
	if !moneyMetric("order_total", "amount") {
		t.Error("amount in the expression should also count as money")
	}
}

func TestCurateFillsBlanksAndNeverWidensWhatSomeoneNarrowed(t *testing.T) {
	m := &semantic.Model{
		Dimensions: []semantic.Dimension{
			{Name: "customer_phone", Entity: "customer", Column: "phone", Type: "categorical"},
			{Name: "store_region", Entity: "store", Column: "region", Type: "categorical"},
			// Already curated by hand — must survive untouched.
			{Name: "member_phone", Entity: "member", Column: "phone", Type: "categorical",
				Synonyms: []string{"mobile"}, Mask: "'hidden'"},
		},
		Metrics: []semantic.Metric{
			{Name: "sale_amount_sum", Entity: "sale", Agg: "sum", Expr: "amount"},
			{Name: "sale_qty_sum", Entity: "sale", Agg: "sum", Expr: "qty"},
			// A hand-narrowed gate must not be replaced by the default one.
			{Name: "sale_cost_sum", Entity: "sale", Agg: "sum", Expr: "cost", Roles: []string{"cfo"}},
		},
	}
	curate(m)

	if m.Dimensions[0].Mask != maskExpr {
		t.Errorf("phone was not masked: %q", m.Dimensions[0].Mask)
	}
	// A mask must carry roles. A mask with no roles means nobody can see it,
	// including the people who should — the compiler now refuses to accept
	// such a model outright, and before it started refusing, every model in
	// this repo was written this way.
	for _, d := range m.Dimensions {
		if d.Mask != "" && len(d.Roles) == 0 {
			t.Errorf("dimension %q has a mask but no roles: this masks it from everyone", d.Name)
		}
	}
	if !has(m.Dimensions[1].Synonyms, "area") {
		t.Errorf("store_region did not get a plain-language synonym: %v", m.Dimensions[1].Synonyms)
	}
	if m.Dimensions[2].Mask != "'hidden'" || !has(m.Dimensions[2].Synonyms, "mobile") {
		t.Error("hand-written mask and synonyms were overwritten")
	}
	if !has(m.Metrics[0].Roles, "finance") {
		t.Errorf("money metric was not gated: %v", m.Metrics[0].Roles)
	}
	if len(m.Metrics[1].Roles) != 0 {
		t.Errorf("quantity metric should not be gated: %v", m.Metrics[1].Roles)
	}
	if len(m.Metrics[2].Roles) != 1 || m.Metrics[2].Roles[0] != "cfo" {
		t.Errorf("a hand-narrowed gate was widened: %v", m.Metrics[2].Roles)
	}
}

// The whole point is that a draft off a live schema is usable in plain
// language and safe by default — not that the helpers pass in isolation.
func TestDraftFromSchemaIsGroundableInPlainLanguageAndSafeByDefault(t *testing.T) {
	schema := &Schema{Tables: []Table{
		{
			Name: "customers", PrimaryKey: "id",
			Columns: []Column{
				{Name: "id", Type: "integer"},
				{Name: "name", Type: "text"},
				{Name: "phone", Type: "text"},
				{Name: "city", Type: "text"},
			},
		},
		{
			Name: "orders", PrimaryKey: "id",
			Columns: []Column{
				{Name: "id", Type: "integer"},
				{Name: "customer_id", Type: "integer"},
				{Name: "amount", Type: "numeric"},
				{Name: "created_at", Type: "timestamp without time zone"},
			},
			ForeignKeys: []ForeignKey{{Column: "customer_id", RefTable: "customers", RefColumn: "id"}},
		},
	}}

	m, err := HeuristicModel(schema)
	if err != nil {
		t.Fatal(err)
	}

	var phone, city *semantic.Dimension
	for i := range m.Dimensions {
		switch m.Dimensions[i].Column {
		case "phone":
			phone = &m.Dimensions[i]
		case "city":
			city = &m.Dimensions[i]
		}
	}
	if phone == nil || phone.Mask == "" {
		t.Error("the phone column was drafted without a mask — a leak is invisible, over-masking is a one-line fix")
	}
	if city == nil || !has(city.Synonyms, "town") {
		t.Error("the city dimension has no plain-language synonym, so a plain-language question can't ground on it")
	}
	// created_at isn't in the lexicon — it falls back on type=time, because
	// enumerating every spelling of a timestamp column is a losing game.
	for _, d := range m.Dimensions {
		if d.Column == "created_at" && !has(d.Synonyms, "date") {
			t.Errorf("time dimension has no plain-language synonym: %v", d.Synonyms)
		}
	}

	var gatedMoney, openQty bool
	for _, mt := range m.Metrics {
		if strings.Contains(mt.Name, "amount") && has(mt.Roles, "finance") {
			gatedMoney = true
		}
		if strings.HasSuffix(mt.Name, "_count") && len(mt.Roles) == 0 {
			openQty = true
		}
	}
	if !gatedMoney {
		t.Error("money metric was not gated to finance by default")
	}
	if !openQty {
		t.Error("count metric was wrongly gated — a count should be visible to everyone")
	}
}
