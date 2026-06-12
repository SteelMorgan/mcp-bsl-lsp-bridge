package tools

import (
	"strings"
	"testing"

	"github.com/myleshyson/lsprotocol-go/protocol"
)

func TestClassifyModuleType(t *testing.T) {
	cases := map[string]string{
		"file:///proj/CommonModules/Расчеты/Ext/Module.bsl":                     "CommonModule",
		"file:///proj/Documents/Заказ/Forms/ФормаДокумента/Ext/Form/Module.bsl": "FormModule",
		"file:///proj/Documents/Заказ/Ext/ManagerModule.bsl":                    "ManagerModule",
		"file:///proj/Documents/Заказ/Ext/ObjectModule.bsl":                     "ObjectModule",
		"file:///proj/AccumulationRegisters/Остатки/Ext/RecordSetModule.bsl":    "RecordSetModule",
		"file:///proj/InformationRegisters/Курсы/Ext/ValueManagerModule.bsl":    "ValueManagerModule",
		"file:///proj/Documents/Заказ/Ext/UnknownModule.bsl":                    "DocumentsModule",
	}
	for uri, want := range cases {
		if got := classifyModuleType(uri); got != want {
			t.Errorf("classifyModuleType(%q) = %q, want %q", uri, got, want)
		}
	}
}

func TestFormatSymbolImpact_GroupsAndSummary(t *testing.T) {
	items := []protocol.CallHierarchyItem{{Name: "МойМетод"}}
	callers := []protocol.CallHierarchyIncomingCall{
		{
			From:       protocol.CallHierarchyItem{Name: "ВызовИзОбщего", Uri: "file:///CommonModules/X/Ext/Module.bsl"},
			FromRanges: []protocol.Range{{}},
		},
		{
			From:       protocol.CallHierarchyItem{Name: "ВызовИзФормы", Uri: "file:///Documents/Z/Forms/Ф/Ext/Form/Module.bsl"},
			FromRanges: []protocol.Range{{}, {}},
		},
	}
	refs := []protocol.Location{
		{Uri: "file:///CommonModules/X/Ext/Module.bsl", Range: protocol.Range{Start: protocol.Position{Line: 3}}},
		{Uri: "file:///Documents/Z/Forms/Ф/Ext/Form/Module.bsl", Range: protocol.Range{Start: protocol.Position{Line: 9}}},
	}

	out := formatSymbolImpact("МойМетод", "file:///m.bsl", 1, 1, items, callers, refs, nil)

	for _, want := range []string{
		"SYMBOL IMPACT: МойМетод",
		"INCOMING CALLERS (2)",
		"CommonModule:1",
		"FormModule:1",
		"REFERENCES (2",
		"IMPACT SUMMARY: 2 direct caller(s) across 2 module type(s), 2 reference site(s)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q:\n%s", want, out)
		}
	}
}

func TestFormatSymbolImpact_NoSymbol(t *testing.T) {
	out := formatSymbolImpact("", "file:///m.bsl", 0, 0, nil, nil, nil, nil)
	if !strings.Contains(out, "No call-hierarchy symbol") {
		t.Errorf("expected no-symbol message:\n%s", out)
	}
}
