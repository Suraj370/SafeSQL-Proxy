package modelgen

import (
	"slices"
	"strings"

	semantic "github.com/liliang-cn/semantic-go"
)

// Curation — the three things a heuristic draft used to leave blank, and which
// are exactly the things that decide whether the draft is usable on site.
//
// The structural half of a draft (entities, joins, cardinality, grain) is
// derivable from the schema, and HeuristicModel gets it right. The governance
// half is not derivable, so it came out empty: `synonyms: []`, `mask: ""`,
// `roles: []`. Empty is not neutral. A model with no plain-language synonyms
// cannot be grounded by anyone asking in their own words; a phone column with
// no mask is a phone column in every answer; a metric with no roles is revenue
// that everyone can read. Someone has to fill these in, and "someone will
// review it later" is how they stay empty.
//
// So the draft now guesses, and the guesses are biased. Which way to bias is not
// the same question for all three:
//
//   - Synonyms are purely additive. A wrong one costs a spurious match at worst.
//   - Masking and role-gating are the reverse of the bias used elsewhere in this
//     package (see looksLikeIdentifier: "a missing metric is invisible, a silly
//     one is not"). Here the invisible failure is the harmful one — an unmasked
//     phone number leaks quietly, while an over-masked column produces a
//     complaint within the hour and is deleted in one line. Default closed.
//
// Everything below is derived from the reviewed models running in production,
// not invented: the vocabulary comes from their synonym lists, and the
// money-vs-quantity split from which of their metrics carry `roles:`.

// businessSynonyms maps an English column/metric token to the plain-language
// words a person actually types instead of the schema's snake_case name. Keys
// are matched against a whole name first, then against progressively shorter
// token suffixes, so `store_region` finds `region`.
var businessSynonyms = map[string][]string{
	// —— money ——
	"revenue":   {"top line", "gross sales", "sales revenue"},
	"sales":     {"sales figures", "sales revenue"},
	"amount":    {"total value"},
	"cost":      {"expense"},
	"cogs":      {"cost of goods sold", "cost"},
	"profit":    {"earnings"},
	"margin":    {"gross margin"},
	"price":     {"unit price", "rate"},
	"tax":       {"levy", "duty"},
	"rent":      {"lease payment"},
	"salary":    {"pay", "wages"},
	"wage":      {"pay", "salary"},
	"spend":     {"expenditure", "outlay"},
	"purchase":  {"procurement", "buy"},
	"discount":  {"markdown"},
	"refund":    {"reimbursement"},
	"fee":       {"charge"},
	"utilities": {"utility bills", "power and water"},

	// —— quantity ——
	"qty":          {"quantity"},
	"quantity":     {"amount"},
	"units":        {"pieces", "volume"},
	"headcount":    {"staff count", "employee count"},
	"transactions": {"deals", "txns"},
	"stock":        {"inventory"},
	"inventory":    {"stock"},
	"loss":         {"shrinkage", "wastage"},

	// —— dimensions: place and org ——
	"city":       {"town"},
	"region":     {"area", "territory", "zone"},
	"province":   {"state", "territory"},
	"store":      {"shop", "outlet"},
	"shop":       {"store", "outlet"},
	"clinic":     {"medical office"},
	"hotel":      {"property", "lodge"},
	"factory":    {"plant", "mill"},
	"plant":      {"factory", "site"},
	"warehouse":  {"depot"},
	"line":       {"production line"},
	"dept":       {"division"},
	"department": {"division"},
	"channel":    {"platform", "outlet"},

	// —— dimensions: people and things ——
	"staff":    {"employees"},
	"employee": {"staff"},
	"customer": {"client"},
	"member":   {"subscriber"},
	"guest":    {"visitor"},
	"patient":  {"case"},
	"supplier": {"vendor"},
	"product":  {"item", "goods"},
	"part":     {"component"},
	"sku":      {"item", "goods"},
	"order":    {"purchase order"},
	"room":     {"suite"},

	// —— dimensions: attributes ——
	"category": {"class", "type"},
	"brand":    {"label"},
	"status":   {"state"},
	"type":     {"kind"},
	"tier":     {"level"},
	"grade":    {"rank"},
	"severity": {"seriousness"},
	"position": {"role", "title"},
	"role":     {"title", "position"},
	"shift":    {"work shift"},
	"format":   {"store format"},
	"defect":   {"flaw"},
	"gender":   {"sex"},
	"age":      {"years"},

	// —— time ——
	"date":    {"day"},
	"time":    {"clock time"},
	"month":   {"period"},
	"year":    {"annum"},
	"quarter": {"fiscal quarter"},
	"week":    {"workweek"},
}

// synonymSuffixes turns a trailing token into a word that composes onto the
// base: order + count → "purchase order tally". Only suffixes that read
// naturally when joined onto a noun phrase are listed; anything that would
// read awkwardly is left out rather than produced wrong.
var synonymSuffixes = map[string][]string{
	"count": {"tally", "number"},
	"sum":   {"total"},
	"total": {"grand total"},
	"rate":  {"ratio"},
	"ratio": {"proportion"},
	"avg":   {"average"},
}

// deriveSynonyms proposes plain-language synonyms for a generated dimension or
// metric name. Returns nil when nothing is recognized — an empty list is
// honest, a made-up phrase would not be.
func deriveSynonyms(name string) []string {
	toks := strings.Split(strings.ToLower(name), "_")
	if len(toks) == 0 {
		return nil
	}

	// Whole name first: `gross_margin` is its own phrase, not "margin" composed.
	if v, ok := businessSynonyms[strings.Join(toks, "_")]; ok {
		return append([]string(nil), v...)
	}

	// A trailing count/sum/rate composes onto whatever it counts.
	if len(toks) > 1 {
		if tails, ok := synonymSuffixes[toks[len(toks)-1]]; ok {
			base := deriveSynonyms(strings.Join(toks[:len(toks)-1], "_"))
			if len(base) == 0 {
				return nil
			}
			var out []string
			// `_sum` on a money column reads better bare: "total value", not
			// "total value total".
			if toks[len(toks)-1] == "sum" || toks[len(toks)-1] == "total" {
				out = append(out, base...)
			}
			for _, b := range base {
				for _, t := range tails {
					out = append(out, b+" "+t)
				}
			}
			return dedupe(out, 4)
		}
	}

	// `X_per_Y` must not fall through to the suffix walk: order_amount_per_qty
	// would match on its *denominator* and come back "quantity", which then
	// competes with order_qty_sum for that exact word. Two metrics answering to
	// "quantity" is worse than one metric with no plain-language name — the
	// first silently returns the wrong number, the second visibly returns none.
	// Whole-name entries are already handled above.
	if len(toks) > 2 && slices.Contains(toks[1:len(toks)-1], "per") {
		return nil
	}

	// Otherwise the attribute carries the meaning: store_region → "area", not
	// "store area". Walk from the longest suffix so `id_card` beats `card`.
	for i := range toks {
		if v, ok := businessSynonyms[strings.Join(toks[i:], "_")]; ok {
			return append([]string(nil), v...)
		}
	}
	return nil
}

// piiColumn reports whether a column holds personal data that should not appear
// verbatim in an answer.
//
// Person names are deliberately absent. `customer_name` is PII and `store_name`
// is not, and nothing in a schema distinguishes them; masking both would break
// grouping by store — the single most common thing anyone asks for — to protect
// a column the reviewer can mask in one line. The reviewed models mask phone
// numbers and nothing else, which is the same conclusion reached by hand.
func piiColumn(column string) bool {
	n := strings.ToLower(column)
	for _, frag := range []string{
		"phone", "mobile", "telephone", "email", "e_mail",
		"id_card", "idcard", "id_no", "idno", "identity_no", "ssn", "passport",
		"bank_card", "bankcard", "card_no", "cardno", "account_no", "accountno",
		"address", "addr", "postcode", "zipcode",
	} {
		if strings.Contains(n, frag) {
			return true
		}
	}
	// A bare `tel` column, but not `hotel` / `telecom`.
	return n == "tel" || strings.HasSuffix(n, "_tel")
}

// PIIColumn is piiColumn for the intake plan, which classifies columns before
// any model exists. One rule rather than two: a column the draft model masks
// and the intake plan keeps — or the other way round — would be two answers to
// "is this personal" given by one product.
func PIIColumn(column string) bool { return piiColumn(column) }

// maskExpr is what a masked dimension renders as. A literal, so the column never
// reaches the result set at all.
const maskExpr = `'***'`

// moneyMetric reports whether a metric measures money, and therefore should be
// gated to finance by default.
//
// Rates are split, not lumped: `margin_rate` and `net_margin` are gated in every
// reviewed model because they expose margin, while `yield_rate`, `defect_rate`
// and `on_time_rate` are open in all of them. So the test is the measure, not
// the suffix — a ratio built on money is money.
func moneyMetric(name, expr string) bool {
	hay := strings.ToLower(name + "_" + expr)
	for _, frag := range []string{
		"revenue", "sales", "amount", "price", "cost", "cogs", "profit", "margin",
		"salary", "wage", "payroll", "spend", "payment", "fee", "tax", "rent",
		"gmv", "income", "expense", "budget", "discount", "refund", "deposit",
		"adr", "revpar", "arpu", "aov", "ticket", "turnover",
	} {
		if strings.Contains(hay, frag) {
			return true
		}
	}
	return false
}

// curate fills the governance blanks in a structurally-complete draft:
// plain-language synonyms so the thing can be asked about in everyday words,
// masks on personal data, and a finance gate on money.
//
// It only ever fills blanks. Anything already set — by a hand edit, or by the
// LLM refinement pass — is left alone, so re-running the draft never quietly
// widens a gate somebody narrowed. `admin` rides along on the finance gate so
// whoever administers the deployment is not locked out of their own data.
func curate(m *semantic.Model) {
	for i := range m.Dimensions {
		d := &m.Dimensions[i]
		if len(d.Synonyms) == 0 {
			d.Synonyms = deriveSynonyms(d.Name)
		}
		// A time dimension is asked about as "date" or "time" whatever the
		// column is called. created_at, order_date, biz_dt, ts — enumerating
		// the spellings is a losing game, and the type already settled the
		// question.
		if len(d.Synonyms) == 0 && d.Type == "time" {
			d.Synonyms = []string{"date", "timestamp"}
		}
		if d.Mask == "" && piiColumn(d.Column) {
			d.Mask = maskExpr
		}
		// A mask with no roles is a column nobody can ever see, including the
		// support agent whose job is to phone the customer back. The compiler
		// refuses such a model outright, and that is the right refusal: masking
		// narrows who sees a value, it does not delete it.
		//
		// This runs on any masked dimension, not only the ones masked above: a
		// reviewer who writes `mask:` by hand leaves the same blank, and an
		// empty Roles beside a non-empty Mask is exactly the kind of blank this
		// function exists to fill. The gate is narrower than the money gate
		// below — a leaked phone number costs more than an inconvenienced
		// analyst, and whoever needs it wider widens it in one line.
		if d.Mask != "" && len(d.Roles) == 0 {
			d.Roles = []string{"admin"}
		}
	}
	for i := range m.Metrics {
		mt := &m.Metrics[i]
		mt.Synonyms = dedupe(append(mt.Synonyms, deriveSynonyms(mt.Name)...), 8)
		if len(mt.Roles) == 0 && moneyMetric(mt.Name, mt.Expr+" "+mt.Formula) {
			mt.Roles = []string{"finance", "admin"}
		}
	}
}

func dedupe(in []string, max int) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) == max {
			break
		}
	}
	return out
}
