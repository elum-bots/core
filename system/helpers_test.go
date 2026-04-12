package system

import (
	"strings"
	"testing"
)

func TestHelpInfoTextUsesCustomBlock(t *testing.T) {
	got := helpText(false, Dependencies{
		HelpInfo: "Как пользоваться:\n\nКастомный текст",
	})
	if !strings.Contains(got, "Кастомный текст") {
		t.Fatalf("expected custom help info, got %q", got)
	}
	if strings.Contains(got, "Если есть вопросы — пишите clck.ru/3RFgyG") {
		t.Fatalf("expected default help info to be replaced, got %q", got)
	}
}
