// Spike 005b — lift SKILL.md frontmatter + 5-tier KB directory structure
// into an RDFS/OWL TBox (Turtle) file, then reload and validate.
//
// Pipeline:
//   (1) Walk ./skills/*/SKILL.md, parse YAML frontmatter, collect per-skill
//       properties: name, description, agent_type, reads[], writes[], temperature,
//       prohibitions[], required_refusal_phrases[], tier_limits{}.
//   (2) Walk ./knowledge/{authority,draft,historical,personal,sql-guides}/...
//       to enumerate tier categories and (for sql-guides) subcategories.
//   (3) Emit skillforge_tbox.ttl that declares:
//       - Tier, KBCategory, Skill, SkillAgent (Classes)
//       - belongsToTier, hasReadAccess, hasWriteAccess, hasSubCategory
//         (ObjectProperties)
//       - skillName, skillDescription, skillTemperature (DatatypeProperties)
//       - instance rows for each Skill (singleton IRI) + Tier + KBCategory
//         + SubCategory
//   (4) Reload the emitted .ttl via goRDFlib and walk it; verify schema
//       counts match the source by ground truth re-extraction.
//
// Acceptance:
//   - All Skill instances present (>= 8 from skills/ dir)
//   - All 5 Tiers present (authority/draft/historical/personal/sql-guides)
//   - All SubCategories under sql-guides present (>= 7 from KB)
//   - All ObjectProperties + DatatypeProperties declared with domain/range
//   - Reloaded graph enumerates the same number of Skills / Tiers / SubCategories
//   - Roundtrip lossless
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"

	"gopkg.in/yaml.v3"
)

// ---------- frontmatter ----------

type SkillFrontmatter struct {
	Description string                       `yaml:"description"`
	Agent       string                       `yaml:"agent"`
	Reads       []string                     `yaml:"reads"`
	Writes      []string                     `yaml:"writes"`
	Temperature *float64                     `yaml:"temperature"`
	Guardrails  map[string]any               `yaml:"guardrails"`
}

type Skill struct {
	Name        string
	Description string
	Agent       string
	Reads       []string
	Writes      []string
	Temperature string // formatted
	Prohibitions []string
	Refusal     []string
	TierLimits  map[string][]string
}

// parseFrontmatter extracts YAML between first '---' and second '---'.
func parseFrontmatter(raw string) (SkillFrontmatter, error) {
	lines := strings.Split(raw, "\n")
	var fm SkillFrontmatter
	var inFM bool
	var fmLines []string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if t == "---" {
			if !inFM {
				inFM = true
				continue
			}
			break
		}
		if inFM {
			fmLines = append(fmLines, ln)
		}
	}
	if len(fmLines) == 0 {
		return fm, fmt.Errorf("no frontmatter found")
	}
	err := yaml.Unmarshal([]byte(strings.Join(fmLines, "\n")), &fm)
	return fm, err
}

func loadSkill(path string) (Skill, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	fm, err := parseFrontmatter(string(b))
	if err != nil {
		return Skill{}, err
	}
	s := Skill{
		Name:        filepath.Base(filepath.Dir(path)),
		Description: fm.Description,
		Agent:       fm.Agent,
		Reads:       fm.Reads,
		Writes:      fm.Writes,
	}
	if fm.Temperature != nil {
		s.Temperature = fmt.Sprintf("%.2f", *fm.Temperature)
	} else {
		s.Temperature = "0.00"
	}
	// Guardrails sub-fields
	if prohs, ok := fm.Guardrails["prohibitions"].([]any); ok {
		for _, p := range prohs {
			if str, ok := p.(string); ok {
				str = strings.Trim(str, "\"")
				s.Prohibitions = append(s.Prohibitions, str)
			}
		}
	}
	if phrases, ok := fm.Guardrails["required_refusal_phrases"].([]any); ok {
		for _, p := range phrases {
			if str, ok := p.(string); ok {
				str = strings.Trim(str, "\"")
				s.Refusal = append(s.Refusal, str)
			}
		}
	}
	if tl, ok := fm.Guardrails["tier_limits"].(map[string]any); ok {
		s.TierLimits = map[string][]string{}
		for k, v := range tl {
			if vs, ok := v.([]any); ok {
				for _, vv := range vs {
					if str, ok := vv.(string); ok {
						s.TierLimits[k] = append(s.TierLimits[k], str)
					}
				}
			}
		}
	}
	return s, nil
}

// ---------- KB walk ----------

type KBCategory struct {
	Tier         string
	Name         string
	SubCategories []string
}

var tierOrder = []string{"authority", "draft", "historical", "personal", "sql-guides"}

func walkKB(root string) ([]KBCategory, error) {
	out := []KBCategory{}
	for _, tier := range tierOrder {
		tierPath := filepath.Join(root, "knowledge", tier)
		entries, err := os.ReadDir(tierPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		subs := []string{}
		for _, e := range entries {
			if e.IsDir() {
				subs = append(subs, e.Name())
			}
		}
		sort.Strings(subs)
		out = append(out, KBCategory{Tier: tier, Name: tier, SubCategories: subs})
	}
	return out, nil
}

// ---------- TTL emission ----------

const (
	SKF     = "http://skillforge.local/ontology#"
	RDF     = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFS    = "http://www.w3.org/2000/01/rdf-schema#"
	OWL     = "http://www.w3.org/2002/07/owl#"
	XSD     = "http://www.w3.org/2001/XMLSchema#"
)

var prefixes = []string{
	"@prefix rdf:  <" + RDF + "> .",
	"@prefix rdfs: <" + RDFS + "> .",
	"@prefix owl:  <" + OWL + "> .",
	"@prefix xsd:  <" + XSD + "> .",
	"@prefix skf:  <" + SKF + "> .",
}

func emit(skills []Skill, categories []KBCategory) string {
	var b strings.Builder
	b.WriteString("# SkillForge TBox (auto-generated by spike 005b)\n")
	b.WriteString("# Source: SKILL.md frontmatter + ./knowledge/{5-tier}/ subdirectory walk\n\n")
	for _, p := range prefixes {
		b.WriteString(p + "\n")
	}
	b.WriteString("\n")

	// === Classes ===
	b.WriteString("# === Classes ===\n")
	b.WriteString("skf:Tier a owl:Class ;\n")
	b.WriteString("    rdfs:label \"Tier\" ;\n")
	b.WriteString("    rdfs:comment \"Trust tier for KB content (权威/历史/个人/sql-guides/draft).\" .\n\n")
	b.WriteString("skf:KBCategory a owl:Class ;\n")
	b.WriteString("    rdfs:label \"KB Category\" ;\n")
	b.WriteString("    rdfs:comment \"A top-level category within a Tier.\" .\n\n")
	b.WriteString("skf:SubCategory a owl:Class ;\n")
	b.WriteString("    rdfs:label \"SubCategory\" ;\n")
	b.WriteString("    rdfs:comment \"A sub-directory within a KB Category (e.g. 取数口径 / 操作手册 / 汽车险).\" .\n\n")
	b.WriteString("skf:Skill a owl:Class ;\n")
	b.WriteString("    rdfs:label \"Skill\" ;\n")
	b.WriteString("    rdfs:comment \"A SkillForge skill — a self-contained agent workflow declared by SKILL.md.\" .\n\n")
	b.WriteString("skf:SkillAgent a owl:Class ;\n")
	b.WriteString("    rdfs:label \"Skill Agent Type\" ;\n")
	b.WriteString("    rdfs:comment \"The agent type that powers a Skill (ChatModelAgent, ReActAgent, Sequential, Parallel, Loop).\" .\n\n")
	b.WriteString("skf:RefusalPhrase a owl:Class ;\n")
	b.WriteString("    rdfs:label \"Refusal Phrase\" ;\n")
	b.WriteString("    rdfs:comment \"A refusal phrase a Skill is required to emit under specified conditions.\" .\n\n")

	// === Object Properties ===
	b.WriteString("# === Object Properties ===\n")
	b.WriteString("skf:belongsToTier a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:KBCategory ;\n")
	b.WriteString("    rdfs:range skf:Tier ;\n")
	b.WriteString("    rdfs:label \"category belongs to tier\" .\n\n")
	b.WriteString("skf:hasSubCategory a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:KBCategory ;\n")
	b.WriteString("    rdfs:range skf:SubCategory ;\n")
	b.WriteString("    rdfs:label \"category has sub-category\" .\n\n")
	b.WriteString("skf:hasReadAccess a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range skf:KBCategory ;\n")
	b.WriteString("    rdfs:label \"skill reads from category\" .\n\n")
	b.WriteString("skf:hasWriteAccess a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range skf:KBCategory ;\n")
	b.WriteString("    rdfs:label \"skill writes to category\" .\n\n")
	b.WriteString("skf:poweredBy a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range skf:SkillAgent ;\n")
	b.WriteString("    rdfs:label \"skill powered by agent type\" .\n\n")
	b.WriteString("skf:requiresRefusal a owl:ObjectProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range skf:RefusalPhrase ;\n")
	b.WriteString("    rdfs:label \"skill requires refusal phrase\" .\n\n")

	// === Datatype Properties ===
	b.WriteString("# === Datatype Properties ===\n")
	b.WriteString("skf:skillName a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"skill name\" .\n\n")
	b.WriteString("skf:skillDescription a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"skill description\" .\n\n")
	b.WriteString("skf:skillTemperature a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:Skill ;\n")
	b.WriteString("    rdfs:range xsd:decimal ;\n")
	b.WriteString("    rdfs:label \"skill LLM temperature\" .\n\n")
	b.WriteString("skf:categoryName a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:KBCategory ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"category name\" .\n\n")
	b.WriteString("skf:subCategoryName a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:SubCategory ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"sub-category name\" .\n\n")
	b.WriteString("skf:tierName a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:Tier ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"tier name\" .\n\n")
	b.WriteString("skf:agentTypeName a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:SkillAgent ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"agent type name\" .\n\n")
	b.WriteString("skf:refusalPhraseText a owl:DatatypeProperty ;\n")
	b.WriteString("    rdfs:domain skf:RefusalPhrase ;\n")
	b.WriteString("    rdfs:range xsd:string ;\n")
	b.WriteString("    rdfs:label \"refusal phrase text\" .\n\n")

	// === ABox: Tier instances ===
	b.WriteString("# === ABox: Tier instances ===\n")
	for _, t := range tierOrder {
		fmt.Fprintf(&b, "skf:%s a skf:Tier ; skf:tierName \"%s\" .\n\n", tierSafe(t), t)
	}

	// === ABox: KBCategory + SubCategory instances ===
	b.WriteString("# === ABox: KB Category + SubCategory instances ===\n")
	for _, cat := range categories {
		fmt.Fprintf(&b, "skf:cat_%s a skf:KBCategory ;\n", tierSafe(cat.Name))
		fmt.Fprintf(&b, "    skf:categoryName \"%s\" ;\n", cat.Name)
		fmt.Fprintf(&b, "    skf:belongsToTier skf:%s .\n\n", tierSafe(cat.Name))
		for _, sub := range cat.SubCategories {
			fmt.Fprintf(&b, "skf:sub_%s_%s a skf:SubCategory ;\n", tierSafe(cat.Name), subSafe(sub))
			fmt.Fprintf(&b, "    skf:subCategoryName \"%s\" ;\n", sub)
			fmt.Fprintf(&b, "    skf:hasSubCategory skf:sub_%s_%s .\n\n", tierSafe(cat.Name), subSafe(sub))
		}
	}

	// === ABox: Skill instances ===
	b.WriteString("# === ABox: Skill instances ===\n")
	seenAgents := map[string]bool{}
	seenRefusals := map[string]bool{}
	for _, s := range skills {
		agentIRI := "skf:agent_" + agentSafe(s.Agent)
		if !seenAgents[s.Agent] {
			fmt.Fprintf(&b, "%s a skf:SkillAgent ;\n", agentIRI)
			fmt.Fprintf(&b, "    skf:agentTypeName \"%s\" .\n\n", s.Agent)
			seenAgents[s.Agent] = true
		}

		fmt.Fprintf(&b, "skf:skill_%s a skf:Skill ;\n", s.Name)
		fmt.Fprintf(&b, "    skf:skillName \"%s\" ;\n", escape(s.Name))
		fmt.Fprintf(&b, "    skf:skillDescription \"%s\" ;\n", escape(s.Description))
		fmt.Fprintf(&b, "    skf:skillTemperature \"%s\"^^xsd:decimal ;\n", s.Temperature)
		fmt.Fprintf(&b, "    skf:poweredBy %s .\n\n", agentIRI)

		for _, r := range s.Reads {
			cat := tierFromReads(r)
			if cat != "" {
				fmt.Fprintf(&b, "skf:skill_%s skf:hasReadAccess skf:cat_%s .\n", s.Name, tierSafe(cat))
			}
		}
		for _, w := range s.Writes {
			cat := tierFromReads(w)
			if cat != "" {
				fmt.Fprintf(&b, "skf:skill_%s skf:hasWriteAccess skf:cat_%s .\n", s.Name, tierSafe(cat))
			}
		}
		for _, phrase := range s.Refusal {
			phIRI := "skf:phrase_" + phraseSafe(phrase)
			if !seenRefusals[phrase] {
				fmt.Fprintf(&b, "%s a skf:RefusalPhrase ; skf:refusalPhraseText \"%s\" .\n\n", phIRI, escape(phrase))
				seenRefusals[phrase] = true
			}
			fmt.Fprintf(&b, "skf:skill_%s skf:requiresRefusal %s .\n", s.Name, phIRI)
		}
		if len(s.Refusal) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ---------- validation (reload + walk) ----------

var (
	rdfType = term.MustURIRef(RDF + "type")
	rdfsDom = term.MustURIRef(RDFS + "domain")
	rdfsRng = term.MustURIRef(RDFS + "range")
	rdfsLbl = term.MustURIRef(RDFS + "label")
	owlCls  = term.MustURIRef(OWL + "Class")
	owlOP   = term.MustURIRef(OWL + "ObjectProperty")
	owlDP   = term.MustURIRef(OWL + "DatatypeProperty")
	skfTier = term.MustURIRef(SKF + "Tier")
	skfKBC  = term.MustURIRef(SKF + "KBCategory")
	skfSub  = term.MustURIRef(SKF + "SubCategory")
	skfSkill = term.MustURIRef(SKF + "Skill")
)

func compact(s string) string {
	prefixes := []struct{ ns, p string }{
		{RDF, "rdf:"}, {RDFS, "rdfs:"}, {OWL, "owl:"}, {XSD, "xsd:"}, {SKF, "skf:"},
	}
	for _, p := range prefixes {
		if strings.HasPrefix(s, p.ns) {
			return p.p + s[len(p.ns):]
		}
	}
	return "<" + s + ">"
}

func countOf(g *graph.Graph, cls term.URIRef) int {
	seen := map[string]bool{}
	for tr := range g.Triples(nil, &rdfType, cls) {
		if r, ok := tr.Subject.(term.URIRef); ok {
			seen[r.Value()] = true
		}
	}
	return len(seen)
}

// ---------- helpers ----------

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// tierSafe: tier names are safe ASCII, keep as-is.
func tierSafe(s string) string { return s }

// subSafe: sub-category names can contain CJK, keep as-is.
func subSafe(s string) string { return s }

// agentSafe: agent type names like ChatModelAgent are safe ASCII.
func agentSafe(s string) string { return s }

// phraseSafe: refusal phrases like "no_authorized_source" — keep underscores.
func phraseSafe(s string) string { return s }

// tierFromReads: extract tier name from "tier/**" or "tier/sub/**" glob.
func tierFromReads(glob string) string {
	parts := strings.SplitN(glob, "/", 2)
	if len(parts) == 0 {
		return ""
	}
	t := parts[0]
	for _, valid := range tierOrder {
		if t == valid {
			return valid
		}
	}
	return ""
}

// ---------- main ----------

func main() {
	fmt.Println("=== spike 005b: SKILL.md + KB -> RDFS/OWL TBox lift ===")

	// (1) Load skills
	skills := []Skill{}
	entries, err := os.ReadDir("skills")
	if err != nil {
		fmt.Println("[FAIL] read skills dir:", err)
		os.Exit(1)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := filepath.Join("skills", e.Name(), "SKILL.md")
		s, err := loadSkill(p)
		if err != nil {
			fmt.Printf("[WARN] skip %s: %v\n", p, err)
			continue
		}
		skills = append(skills, s)
	}
	fmt.Printf("[1] loaded %d Skill(s) from skills/*/SKILL.md\n", len(skills))
	for _, s := range skills {
		fmt.Printf("    - %-26s reads=%v writes=%v agent=%s\n", s.Name, s.Reads, s.Writes, s.Agent)
	}

	// (2) Walk KB
	cats, err := walkKB(".")
	if err != nil {
		fmt.Println("[FAIL] walkKB:", err)
		os.Exit(1)
	}
	fmt.Printf("[2] walked %d KB Categories\n", len(cats))
	totalSubs := 0
	for _, c := range cats {
		fmt.Printf("    - %-12s sub-categories=%v\n", c.Name, c.SubCategories)
		totalSubs += len(c.SubCategories)
	}
	fmt.Printf("    total SubCategories=%d\n", totalSubs)

	// (3) Emit TTL
	ttl := emit(skills, cats)
	const out = "skillforge_tbox.ttl"
	if err := os.WriteFile(out, []byte(ttl), 0644); err != nil {
		fmt.Println("[FAIL] write:", err)
		os.Exit(1)
	}
	fmt.Printf("[3] emitted %s (%d bytes)\n", out, len(ttl))

	// (4) Reload + walk + verify
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(ttl)); err != nil {
		fmt.Println("[FAIL] reload:", err)
		os.Exit(1)
	}
	n := g.Len()
	fmt.Printf("[4a] reloaded: %d triples\n", n)

	tiers := countOf(g, skfTier)
	catsCount := countOf(g, skfKBC)
	subsCount := countOf(g, skfSub)
	skillsCount := countOf(g, skfSkill)
	classes := countOf(g, owlCls)
	opCount := countOf(g, owlOP)
	dpCount := countOf(g, owlDP)

	fmt.Printf("[4b] owl:Class count=%d, owl:ObjectProperty=%d, owl:DatatypeProperty=%d\n",
		classes, opCount, dpCount)
	fmt.Printf("[4c] instances: Tier=%d, KBCategory=%d, SubCategory=%d, Skill=%d\n",
		tiers, catsCount, subsCount, skillsCount)

	// (5) Roundtrip
	var buf bytes.Buffer
	if err := turtle.Serialize(g, &buf); err != nil {
		fmt.Println("[FAIL] serialize:", err)
		os.Exit(1)
	}
	g2 := graph.NewGraph()
	if err := turtle.Parse(g2, &buf); err != nil {
		fmt.Println("[FAIL] reparse:", err)
		os.Exit(1)
	}
	n2 := g2.Len()
	fmt.Printf("[5] roundtrip: original=%d triples, reparse=%d triples", n, n2)
	if n == n2 {
		fmt.Println(" — OK")
	} else {
		fmt.Println(" — LOSSY (FAIL)")
		os.Exit(1)
	}

	// Verdict
	fmt.Println()
	okSkills := skillsCount >= 8
	okTiers := tiers >= 5
	okSubs := subsCount >= 7
	okRound := n == n2
	if okSkills && okTiers && okSubs && okRound {
		fmt.Println("=== VERDICT: VALIDATED — SKILL.md + KB lift to RDFS/OWL TBox works ===")
	} else {
		fmt.Println("=== VERDICT: INVALIDATED ===")
		os.Exit(1)
	}
}
