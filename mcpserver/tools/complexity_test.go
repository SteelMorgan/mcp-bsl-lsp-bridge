package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestParseComplexityValue(t *testing.T) {
	cases := []struct {
		title string
		want  int
		ok    bool
	}{
		{"Цикломатическая сложность: 8", 8, true},
		{"Когнитивная сложность: 11", 11, true},
		{"Cyclomatic complexity: 0", 0, true},
		{"no number here", 0, false},
		{"", 0, false},
		{"Сложность 123 ", 123, true},
	}
	for _, c := range cases {
		got, ok := parseComplexityValue(c.title)
		if ok != c.ok || got != c.want {
			t.Errorf("parseComplexityValue(%q) = (%d,%v), want (%d,%v)", c.title, got, ok, c.want, c.ok)
		}
	}
}

func lens(line uint32, method, id, title string) protocol.CodeLens {
	return protocol.CodeLens{
		Range:   protocol.Range{Start: protocol.Position{Line: line}},
		Command: &protocol.Command{Title: title},
		Data:    map[string]interface{}{"methodName": method, "id": id},
	}
}

func TestFormatComplexity_FoldsAndFlags(t *testing.T) {
	lenses := []protocol.CodeLens{
		lens(0, "СложныйМетод", "cognitiveComplexity", "Когнитивная сложность: 18"),
		lens(0, "СложныйМетод", "cyclomaticComplexity", "Цикломатическая сложность: 25"),
		lens(18, "Простой", "cognitiveComplexity", "Когнитивная сложность: 1"),
		lens(18, "Простой", "cyclomaticComplexity", "Цикломатическая сложность: 2"),
	}
	out := formatComplexity(lenses, "file:///m.bsl")

	if !strings.Contains(out, "СложныйМетод (1)") {
		t.Errorf("expected method row with 1-based line, got:\n%s", out)
	}
	// СложныйМетод: cyc 25 > 20 and cog 18 > 15 -> flagged
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected a flag for over-threshold method, got:\n%s", out)
	}
	if !strings.Contains(out, "Methods: 2") {
		t.Errorf("expected 2 methods, got:\n%s", out)
	}
	if !strings.Contains(out, "Over threshold: 1") {
		t.Errorf("expected 1 over-threshold method, got:\n%s", out)
	}
	if !strings.Contains(out, "Max cyclomatic: 25") || !strings.Contains(out, "Max cognitive: 18") {
		t.Errorf("expected max values in summary, got:\n%s", out)
	}
}

func TestFormatComplexity_Empty(t *testing.T) {
	out := formatComplexity(nil, "file:///m.bsl")
	if !strings.Contains(out, "No complexity metrics") {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}
