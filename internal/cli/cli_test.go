package cli

import (
	"io"
	"os"
	"strings"
	"testing"

	"gcetools/internal/i18n"
)

// The usage blocks are translated as a whole, so each must be a dictionary
// entry (an exact copy of the Spanish text).
func TestUsageBlocksTranslated(t *testing.T) {
	for name, s := range map[string]string{"help": help, "usage": usage, "diffUsage": diffUsage, "loginUsage": loginUsage, "rulesHelp": rulesHelp} {
		if !i18n.Has(s) {
			t.Errorf("%s has no English entry (did the Spanish text change?)", name)
		}
	}
}

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	f()
	os.Stdout = old
	w.Close()
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestHelpInEnglish(t *testing.T) {
	defer i18n.SetLang("es")
	out := captureStdout(t, func() {
		if code, ok := Dispatch([]string{"--lang", "en", "help"}); !ok || code != 0 {
			t.Errorf("code=%d ok=%v", code, ok)
		}
	})
	for _, want := range []string{"Commands:", "scan projects and export the inventory", "--lang es|en"} {
		if !strings.Contains(out, want) {
			t.Errorf("English help lacks %q:\n%s", want, out)
		}
	}
	if !strings.Contains(i18n.T(usage), "Usage:\n  gcetools scan [options]") {
		t.Error("scan usage not in English")
	}

	out = captureStdout(t, func() { Dispatch([]string{"--lang=es", "help"}) })
	if !strings.Contains(out, "Comandos:") {
		t.Errorf("Spanish help:\n%s", out)
	}
}
