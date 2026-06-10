package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestEmbeddedDiagnosticTable(t *testing.T) {
	if len(bslDiagnosticTable) < 150 {
		t.Fatalf("embedded diagnostic table looks too small: %d entries", len(bslDiagnosticTable))
	}
	// Spot-check a representative rule from each target category.
	if m, ok := bslDiagnosticTable["CreateQueryInCycle"]; !ok || !strInSlice(m.Tags, "PERFORMANCE") {
		t.Errorf("CreateQueryInCycle must be tagged PERFORMANCE, got %+v", m)
	}
	if m, ok := bslDiagnosticTable["DisableSafeMode"]; !ok || m.Type != "VULNERABILITY" {
		t.Errorf("DisableSafeMode must be VULNERABILITY, got %+v", m)
	}
	if m, ok := bslDiagnosticTable["AssignAliasFieldsInQuery"]; !ok || !strInSlice(m.Tags, "SQL") {
		t.Errorf("AssignAliasFieldsInQuery must be tagged SQL, got %+v", m)
	}
}

func TestQualityCategories(t *testing.T) {
	sec := qualityCategories(bslDiagnosticMeta{Type: "SECURITY_HOTSPOT"})
	if !strInSlice(sec, "security") {
		t.Errorf("SECURITY_HOTSPOT -> security, got %v", sec)
	}
	perfSql := qualityCategories(bslDiagnosticMeta{Tags: []string{"PERFORMANCE", "SQL"}})
	if !strInSlice(perfSql, "performance") || !strInSlice(perfSql, "sql") {
		t.Errorf("PERFORMANCE+SQL tags -> both categories, got %v", perfSql)
	}
	none := qualityCategories(bslDiagnosticMeta{Type: "CODE_SMELL", Tags: []string{"STANDARD"}})
	if len(none) != 0 {
		t.Errorf("plain code-smell should map to no quality category, got %v", none)
	}
}

func TestParseCategories(t *testing.T) {
	all := parseCategories("")
	if !(all["security"] && all["performance"] && all["sql"]) {
		t.Errorf("empty -> all categories, got %v", all)
	}
	one := parseCategories("performance")
	if one["security"] || one["sql"] || !one["performance"] {
		t.Errorf("'performance' -> only performance, got %v", one)
	}
	garbage := parseCategories("nonsense") // falls back to all
	if !(garbage["security"] && garbage["performance"] && garbage["sql"]) {
		t.Errorf("unrecognized -> all categories, got %v", garbage)
	}
}

func diag(line uint32, code, msg string) protocol.Diagnostic {
	return protocol.Diagnostic{
		Range:   protocol.Range{Start: protocol.Position{Line: line}},
		Code:    &protocol.Or2[int32, string]{Value: code},
		Message: msg,
		Source:  "bsl-language-server",
	}
}

func TestFormatQualityDiagnostics_FilterAndGroup(t *testing.T) {
	items := []protocol.Diagnostic{
		diag(0, "CreateQueryInCycle", "запрос в цикле"),       // performance
		diag(5, "DisableSafeMode", "отключение безопасного"),  // security
		diag(9, "AssignAliasFieldsInQuery", "нет псевдонима"), // sql
		diag(12, "MagicNumber", "магическое число"),           // not a quality category -> ignored
		diag(20, "ZzzUnknownRule", "неизвестно"),              // unknown code -> counted as ignored
	}

	// Default: all categories.
	out := formatQualityDiagnostics(items, "file:///m.bsl", parseCategories(""))
	for _, want := range []string{"SECURITY (1)", "PERFORMANCE (1)", "SQL (1)", "Total quality issues: 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "MagicNumber") {
		t.Errorf("MagicNumber must be filtered out:\n%s", out)
	}
	if !strings.Contains(out, "unrecognized codes ignored") {
		t.Errorf("expected unknown-code note:\n%s", out)
	}

	// Filtered: performance only.
	outPerf := formatQualityDiagnostics(items, "file:///m.bsl", parseCategories("performance"))
	if strings.Contains(outPerf, "SECURITY") || strings.Contains(outPerf, "SQL (") {
		t.Errorf("performance filter should drop other categories:\n%s", outPerf)
	}
	if !strings.Contains(outPerf, "CreateQueryInCycle") {
		t.Errorf("performance filter should keep CreateQueryInCycle:\n%s", outPerf)
	}
}

func TestFormatQualityDiagnostics_Clean(t *testing.T) {
	items := []protocol.Diagnostic{diag(0, "MagicNumber", "x")}
	out := formatQualityDiagnostics(items, "file:///m.bsl", parseCategories(""))
	if !strings.Contains(out, "No ") || !strings.Contains(out, "issues found") {
		t.Errorf("expected clean message, got:\n%s", out)
	}
}

func strInSlice(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
