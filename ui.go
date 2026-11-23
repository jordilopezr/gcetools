package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// uiRuntime is the part of the Wails runtime the App uses: events to the
// frontend, native file dialogs and the browser. Tests replace it.
type uiRuntime interface {
	Emit(event string, data ...any)
	OpenFile(opts runtime.OpenDialogOptions) (string, error)
	SaveFile(opts runtime.SaveDialogOptions) (string, error)
	OpenURL(url string)
}

// wailsUI forwards to the Wails runtime of the running app.
type wailsUI struct{ ctx context.Context }

func (w wailsUI) Emit(event string, data ...any) { runtime.EventsEmit(w.ctx, event, data...) }
func (w wailsUI) OpenFile(o runtime.OpenDialogOptions) (string, error) {
	return runtime.OpenFileDialog(w.ctx, o)
}
func (w wailsUI) SaveFile(o runtime.SaveDialogOptions) (string, error) {
	return runtime.SaveFileDialog(w.ctx, o)
}
func (w wailsUI) OpenURL(url string) { runtime.BrowserOpenURL(w.ctx, url) }
