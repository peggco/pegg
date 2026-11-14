package ext

import (
	"github.com/peggco/pegg/internal/core/hook"
	pkgplugin "github.com/peggco/pegg/pkg/plugin"
)

type Page = pkgplugin.TUIPage

var pagesHook = hook.NewHook[[]Page]()

func Register(provider func() []Page) {
	if provider == nil {
		return
	}
	pagesHook.Add(func(in []Page) []Page {
		return append(in, provider()...)
	})
}

func Reset() { pagesHook.Reset() }

func Pages() []Page { return pagesHook.Apply(nil) }
