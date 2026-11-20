package auth

import (
	_ "embed"
	"encoding/base64"
	"html/template"
	"io"
)

//go:embed callback.html
var callbackHTML string

//go:embed callback-icon.svg
var callbackIcon []byte

var callbackTmpl = template.Must(template.New("callback").Parse(callbackHTML))

// writeCallbackPage renders the page shown in the browser after the OAuth
// redirect. detail is the error code returned by Google, if any.
func writeCallbackPage(w io.Writer, ok bool, detail string) error {
	return callbackTmpl.Execute(w, struct {
		OK     bool
		Detail string
		Icon   template.URL
	}{ok, detail, template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(callbackIcon))})
}
