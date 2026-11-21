package auth

import (
	"strings"
	"testing"

	"gcetools/internal/i18n"
)

func TestCallbackPageLanguages(t *testing.T) {
	var b strings.Builder
	if err := writeCallbackPage(&b, true, ""); err != nil {
		t.Fatal(err)
	}
	if s := b.String(); !strings.Contains(s, `<html lang="es">`) || !strings.Contains(s, "<h1>Sesión iniciada</h1>") {
		t.Errorf("Spanish page:\n%s", s)
	}

	i18n.SetLang("en")
	defer i18n.SetLang("es")
	b.Reset()
	if err := writeCallbackPage(&b, true, ""); err != nil {
		t.Fatal(err)
	}
	s := b.String()
	for _, want := range []string{`<html lang="en">`, "<title>gceTools · Signed in</title>", "<h1>Signed in</h1>", "You can close this tab", "only has read access"} {
		if !strings.Contains(s, want) {
			t.Errorf("English success page lacks %q", want)
		}
	}
	b.Reset()
	if err := writeCallbackPage(&b, false, "access_denied"); err != nil {
		t.Fatal(err)
	}
	s = b.String()
	for _, want := range []string{"Sign-in not completed", "<h1>Sign-in was not completed</h1>", `<div class="detail">access_denied</div>`} {
		if !strings.Contains(s, want) {
			t.Errorf("English error page lacks %q", want)
		}
	}
	if strings.Contains(s, "sesión") {
		t.Error("English page still has Spanish text")
	}
}

func TestAuthErrorsInEnglish(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("es")
	_, err := ParseKey([]byte(`{"type":"authorized_user"}`))
	if err == nil || err.Error() != `unsupported credential type "authorized_user": expected "service_account"` {
		t.Errorf("err = %v", err)
	}
	if _, err := ParseOAuthClient([]byte(`{"web":{}}`)); err == nil || !strings.Contains(err.Error(), `"Desktop app"`) {
		t.Errorf("err = %v", err)
	}
}
