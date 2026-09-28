package tools

import (
	"os"
	"path/filepath"
	"testing"

	"rockerboo/mcp-lsp-bridge/mocks"
	"rockerboo/mcp-lsp-bridge/types"

	"github.com/stretchr/testify/mock"
)

// Дефект: workspace/symbol для метода BSL указывает на начало строки
// объявления ("Функция ..."), семантический токен с именем не находится,
// и references запрашиваются на ключевом слове -> пустой результат.
func TestFindPreciseCharacterPositionFallsBackToLineText(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "Module.bsl")
	content := "// заголовок\r\n\tФункция ПересчитатьНаДату(Параметры) Экспорт\r\nКонецФункции\r\n"
	if err := os.WriteFile(file, []byte("\ufeff"+content), 0o600); err != nil {
		t.Fatal(err)
	}
	uri := "file://" + file

	bridge := &mocks.MockBridge{}
	bridge.On("SemanticTokens", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]types.TokenPosition{}, nil)

	// "\tФункция " = 9 UTF-16 единиц
	if got := FindPreciseCharacterPosition(bridge, uri, 1, 0, "ПересчитатьНаДату"); got != 9 {
		t.Fatalf("position = %d, want 9", got)
	}
	// регистр в BSL не важен
	if got := FindPreciseCharacterPosition(bridge, uri, 1, 0, "пересчитатьнадату"); got != 9 {
		t.Fatalf("case-insensitive position = %d, want 9", got)
	}
	// имени нет в строке -> исходная позиция
	if got := FindPreciseCharacterPosition(bridge, uri, 1, 3, "Другое"); got != 3 {
		t.Fatalf("fallback position = %d, want 3", got)
	}
}
