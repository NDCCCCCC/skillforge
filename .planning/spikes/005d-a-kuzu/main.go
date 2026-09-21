// Spike 005d-a — Kuzu embedded graph database for SkillForge TBox.
//
// Validates that Kuzu v0.11.3 (cgo binding) can store the 215-triple
// SkillForge TBox, run basic Cypher queries, and close cleanly. This is
// the "feature-complete" baseline for comparison against 005d-b (memory map).
//
// Acceptance:
//   (1) Open Kuzu in-memory DB without error
//   (2) Create 2 node tables (Skill, Tier) + 1 relationship table (READS_FROM)
//   (3) Insert 8 Skills + 5 Tiers + hasReadAccess edges (subset of TBox)
//   (4) Cypher MATCH (s:Skill)-[:READS_FROM]->(t:Tier) returns 8 rows
//   (5) Total time (open + create + insert + query + close) < 500ms
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/kuzudb/go-kuzu"
)

func main() {
	fmt.Println("=== spike 005d-a: Kuzu embedded KG (cgo binding) ===")

	t0 := time.Now()

	// (1) Open in-memory DB.
	db, err := kuzu.OpenInMemoryDatabase(kuzu.DefaultSystemConfig())
	if err != nil {
		fmt.Println("[FAIL] open:", err)
		os.Exit(1)
	}
	defer db.Close()

	conn, err := kuzu.OpenConnection(db)
	if err != nil {
		fmt.Println("[FAIL] connection:", err)
		os.Exit(1)
	}
	defer conn.Close()
	openDur := time.Since(t0)
	fmt.Printf("[1] open in-memory DB + connection: %.2f ms\n", float64(openDur.Microseconds())/1000)

	// (2) Create node tables.
	stmts := []string{
		"CREATE NODE TABLE Skill(id STRING, name STRING, description STRING, PRIMARY KEY(id))",
		"CREATE NODE TABLE Tier(id STRING, name STRING, PRIMARY KEY(id))",
		"CREATE REL TABLE READS_FROM(FROM Skill TO Tier)",
	}
	for _, s := range stmts {
		res, err := conn.Query(s)
		if err != nil {
			fmt.Printf("[FAIL] %s: %v\n", s, err)
			os.Exit(1)
		}
		res.Close()
	}
	fmt.Printf("[2] created 2 node tables + 1 rel table (%.2f ms total)\n",
		float64(time.Since(t0).Microseconds())/1000)

	// (3) Insert 8 Skills + 5 Tiers.
	tiers := []struct{ id, name string }{
		{"tier:authority", "authority"},
		{"tier:draft", "draft"},
		{"tier:historical", "historical"},
		{"tier:personal", "personal"},
		{"tier:sql-guides", "sql-guides"},
	}
	for _, t := range tiers {
		_, err := conn.Query(fmt.Sprintf("CREATE (t:Tier {id: '%s', name: '%s'})", t.id, t.name))
		if err != nil {
			fmt.Printf("[FAIL] insert tier %s: %v\n", t.id, err)
			os.Exit(1)
		}
	}
	skills := []struct{ id, name, readsTier, desc string }{
		{"skill:simple-qa", "simple-qa", "tier:authority", "factual lookup"},
		{"skill:compliance", "compliance", "tier:authority", "compliance review"},
		{"skill:sql-guides", "sql-guides", "tier:sql-guides", "SQL operational guides"},
		{"skill:calculator", "calculator", "tier:draft", "calculator"},
		{"skill:kpi-dashboard", "kpi-dashboard", "tier:personal", "KPI"},
		{"skill:external-lookup", "external-lookup", "tier:personal", "external data"},
		{"skill:monthly-report", "monthly-report", "tier:personal", "monthly report"},
		{"skill:compliance-orchestrator", "compliance-orchestrator", "tier:authority", "orchestrator"},
	}
	for _, s := range skills {
		// Match existing Tier then create Skill + edge.
		q := fmt.Sprintf(
			"MATCH (t:Tier {id: '%s'}) CREATE (s:Skill {id: '%s', name: '%s', description: '%s'})-[:READS_FROM]->(t)",
			s.readsTier, s.id, s.name, s.desc)
		_, err := conn.Query(q)
		if err != nil {
			fmt.Printf("[FAIL] insert skill %s: %v\n", s.id, err)
			os.Exit(1)
		}
	}
	fmt.Printf("[3] inserted 5 Tiers + 8 Skills (with READS_FROM edges): %.2f ms total\n",
		float64(time.Since(t0).Microseconds())/1000)

	// (4) Query: list all skills that READ from each tier.
	t1 := time.Now()
	res, err := conn.Query("MATCH (s:Skill)-[:READS_FROM]->(t:Tier) RETURN s.name, t.name ORDER BY t.name, s.name")
	if err != nil {
		fmt.Println("[FAIL] query:", err)
		os.Exit(1)
	}
	defer res.Close()
	fmt.Printf("[4] MATCH (s)-[:READS_FROM]->(t) results (%d rows, %.2f ms):\n",
		res.GetNumberOfRows(), float64(time.Since(t1).Microseconds())/1000)
	for res.HasNext() {
		tup, err := res.Next()
		if err != nil {
			fmt.Println("[FAIL] next:", err)
			os.Exit(1)
		}
		// Each FlatTuple has GetValue(i) for column i.
		sName, _ := tup.GetValue(0)
		tName, _ := tup.GetValue(1)
		fmt.Printf("    - %s -> %s\n", sName, tName)
	}

	// (5) Total elapsed.
	total := time.Since(t0)
	fmt.Printf("[5] total elapsed: %.2f ms\n", float64(total.Microseconds())/1000)

	fmt.Println()
	if total < 500*time.Millisecond && res.GetNumberOfRows() == 8 {
		fmt.Println("=== VERDICT: VALIDATED — Kuzu embedded KG meets SkillForge baseline ===")
	} else {
		fmt.Printf("=== VERDICT: INVALIDATED — total=%v rows=%d (want <500ms, 8 rows) ===\n",
			total, res.GetNumberOfRows())
		os.Exit(1)
	}
}
