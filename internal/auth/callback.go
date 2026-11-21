package auth

import (
	_ "embed"
	"encoding/base64"
	"html/template"
	"io"

	"gcetools/internal/i18n"
)

//go:embed callback.html
var callbackHTML string

//go:embed callback-icon.svg
var callbackIcon []byte

var callbackTmpl = template.Must(template.New("callback").Parse(callbackHTML))

// writeCallbackPage renders the page shown in the browser after the OAuth
// redirect, in the active language. detail is the error code returned by
// Google, if any (shown as is).
func writeCallbackPage(w io.Writer, ok bool, detail string) error {
	data := struct {
		OK                                  bool
		Lang, Title, Heading, Message, Foot string
		Detail                              string
		Icon                                template.URL
	}{
		OK:     ok,
		Lang:   i18n.Lang(),
		Foot:   i18n.T("gceTools solo tiene acceso de lectura a Google Cloud. Puedes revocarlo en cualquier momento desde «Cerrar sesión»."),
		Detail: detail,
		Icon:   template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(callbackIcon)),
	}
	if ok {
		data.Title = i18n.T("Sesión iniciada")
		data.Heading = data.Title
		data.Message = i18n.T("Ya puedes cerrar esta pestaña y volver a gceTools.")
	} else {
		data.Title = i18n.T("Inicio de sesión no completado")
		data.Heading = i18n.T("No se completó el inicio de sesión")
		data.Message = i18n.T("Vuelve a gceTools e inténtalo de nuevo.")
	}
	return callbackTmpl.Execute(w, data)
}
