package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func ckind(k protocol.CompletionItemKind) *protocol.CompletionItemKind { return &k }

func TestFormatCompletionItems_DetailAndDeprecated(t *testing.T) {
	list := &protocol.CompletionList{
		Items: []protocol.CompletionItem{
			{Label: "Добавить", Kind: ckind(protocol.CompletionItemKindMethod), Detail: "(): СтрокаТаблицыЗначений"},
			{Label: "Колонки", Kind: ckind(protocol.CompletionItemKindProperty), Detail: "КоллекцияКолонокТаблицыЗначений"},
			{Label: "Устаревший", Kind: ckind(protocol.CompletionItemKindMethod), Tags: []protocol.CompletionItemTag{protocol.CompletionItemTagDeprecated}},
		},
	}
	out := formatCompletionItems(list, "file:///m.bsl", 4, 8, 100)

	if !strings.Contains(out, "Добавить") || !strings.Contains(out, ": СтрокаТаблицыЗначений") {
		t.Errorf("expected method with return-type detail:\n%s", out)
	}
	if !strings.Contains(out, "[property") || !strings.Contains(out, "КоллекцияКолонокТаблицыЗначений") {
		t.Errorf("expected property with its type:\n%s", out)
	}
	if !strings.Contains(out, "[DEPRECATED]") {
		t.Errorf("expected deprecated marker:\n%s", out)
	}
	if !strings.Contains(out, "3 item(s)") {
		t.Errorf("expected count of 3:\n%s", out)
	}
}

func TestFormatCompletionItems_Truncation(t *testing.T) {
	items := make([]protocol.CompletionItem, 0, 250)
	for i := 0; i < 250; i++ {
		items = append(items, protocol.CompletionItem{Label: "Item" + string(rune('A'+i%26)), Kind: ckind(protocol.CompletionItemKindVariable)})
	}
	out := formatCompletionItems(&protocol.CompletionList{Items: items}, "file:///m.bsl", 0, 0, 50)
	if !strings.Contains(out, "showing 50") || !strings.Contains(out, "200 more not shown") {
		t.Errorf("expected truncation note (250 total, limit 50):\n%s", out[:200])
	}
}

func TestFormatCompletionItems_Empty(t *testing.T) {
	out := formatCompletionItems(&protocol.CompletionList{}, "file:///m.bsl", 1, 1, 100)
	if !strings.Contains(out, "No completions") {
		t.Errorf("expected empty message, got:\n%s", out)
	}
}
