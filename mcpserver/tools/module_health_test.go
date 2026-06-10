package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

// reuses lens() from complexity_test.go and diag() from quality_diagnostics_test.go (same package).

func TestFormatModuleHealth_RanksAndAttributes(t *testing.T) {
	lenses := []protocol.CodeLens{
		lens(0, "МетодA", "cyclomaticComplexity", "Цикломатическая сложность: 25"), // over (>20)
		lens(0, "МетодA", "cognitiveComplexity", "Когнитивная сложность: 5"),
		lens(50, "МетодB", "cyclomaticComplexity", "Цикломатическая сложность: 3"),
		lens(50, "МетодB", "cognitiveComplexity", "Когнитивная сложность: 2"),
	}
	report := &protocol.DocumentDiagnosticReport{
		Value: protocol.RelatedFullDocumentDiagnosticReport{
			Items: []protocol.Diagnostic{
				diag(10, "CommonModuleNameFullAccess", "security at МетодA"), // security → МетодA (line 0 <= 10)
				diag(55, "CreateQueryInCycle", "query in loop at МетодB"),    // performance → МетодB (50 <= 55)
			},
		},
	}

	out := formatModuleHealth(lenses, report, "file:///CommonModules/X/Ext/Module.bsl")

	if !strings.Contains(out, "MODULE HEALTH") || !strings.Contains(out, "TOP REFACTOR TARGETS") {
		t.Fatalf("missing headers:\n%s", out)
	}
	// МетодA (score 25+10=35) must rank above МетодB (score 4).
	ia, ib := strings.Index(out, "МетодA"), strings.Index(out, "МетодB")
	if ia == -1 || ib == -1 || ia > ib {
		t.Errorf("expected МетодA ranked before МетодB:\n%s", out)
	}
	if !strings.Contains(out, "security:1") {
		t.Errorf("expected security issue attributed to a method:\n%s", out)
	}
	if !strings.Contains(out, "over threshold: 1") {
		t.Errorf("expected one method over threshold:\n%s", out)
	}
}

func TestFormatModuleHealth_CleanModule(t *testing.T) {
	lenses := []protocol.CodeLens{
		lens(0, "Чистый", "cyclomaticComplexity", "Цикломатическая сложность: 2"),
		lens(0, "Чистый", "cognitiveComplexity", "Когнитивная сложность: 1"),
	}
	report := &protocol.DocumentDiagnosticReport{
		Value: protocol.RelatedFullDocumentDiagnosticReport{Items: []protocol.Diagnostic{}},
	}
	out := formatModuleHealth(lenses, report, "file:///m.bsl")
	if !strings.Contains(out, "✅ none") {
		t.Errorf("expected empty top-targets marker for a clean module:\n%s", out)
	}
}

func TestEnclosingMethod(t *testing.T) {
	rows := []*healthRow{{method: "A", line: 0}, {method: "B", line: 50}}
	if m := enclosingMethod(rows, 10); m == nil || m.method != "A" {
		t.Errorf("line 10 should map to A, got %v", m)
	}
	if m := enclosingMethod(rows, 60); m == nil || m.method != "B" {
		t.Errorf("line 60 should map to B, got %v", m)
	}
	if m := enclosingMethod([]*healthRow{{method: "A", line: 5}}, 2); m != nil {
		t.Errorf("line before first method should be module-level (nil), got %v", m)
	}
}
