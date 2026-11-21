package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"

	"gcetools/internal/auth"
	"gcetools/internal/i18n"
)

const loginUsage = `gcetools login — inicia sesión con Google sin gcloud

Uso:
  gcetools login [--client client_secret_….json]

Usa el cliente OAuth "App de escritorio" de tu organización (ver README,
"Iniciar sesión con Google"). El cliente y el token se guardan en el llavero
del sistema; después: gcetools scan --auth oauth

Opciones:
`

func Login(args []string) int {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), i18n.T(loginUsage)); fs.PrintDefaults() }
	client := fs.String("client", "", i18n.T("JSON del cliente OAuth de tu organización (solo la primera vez o para cambiarlo)"))
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *client != "" {
		data, err := os.ReadFile(*client)
		if err != nil {
			return fail(err.Error())
		}
		c, err := auth.ParseOAuthClient(data)
		if err != nil {
			return fail(err.Error())
		}
		if err := auth.SetOAuthClient(c); err != nil {
			return fail(err.Error())
		}
	}
	if st := auth.GetOAuthStatus(); st.Error != "" {
		return fail(st.Error)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	account, err := auth.Login(ctx, func(u string) {
		fmt.Fprint(os.Stderr, i18n.Tf("Abre este enlace para iniciar sesión (se intenta abrir solo):\n\n  %s\n\nEsperando…\n", u))
		openBrowser(u)
	})
	if err != nil {
		return fail(err.Error())
	}
	fmt.Fprint(os.Stderr, i18n.Tf("Sesión iniciada como %s. Usa: gcetools scan --auth oauth\n", account))
	return 0
}

func Logout(args []string) int {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	all := fs.Bool("all", false, i18n.T("olvida también el cliente OAuth de la organización"))
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := auth.ForgetOAuth(*all); err != nil {
		return fail(err.Error())
	}
	fmt.Fprintln(os.Stderr, i18n.T("Sesión de Google cerrada."))
	return 0
}

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	_ = cmd.Start() // best effort: the URL is printed anyway
}
