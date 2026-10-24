package views

import "log"

// localeChoices are the languages the app is written in, in the order the
// language's popover lists them.
var localeChoices = []string{"zh-Hans", "ja", "en"}

// InitLocale sets the language of the first snapshot before the window's
// first frame; the rebuild is cheap enough for startup.
func (a *App) InitLocale(tag string) {
	if tag == "" {
		tag = "zh-Hans"
	}
	if tag == a.lang {
		return
	}
	a.lang = tag
	if a.opt.Catalog == nil {
		return
	}
	a.opt.Catalog.SetLocale(tag)
	a.opt.Catalog.Rebuild()
}

// ChooseLocale saves and applies an interface language; "" follows the
// system. The window redraws once MyGo reports the resolved language.
func (a *App) ChooseLocale(tag string) {
	a.langChoice = tag
	a.langOpen = false
	if a.opt.SaveLocale != nil {
		if err := a.opt.SaveLocale(tag); err != nil {
			log.Printf("locale.json: %v", err)
		}
	}
	if a.opt.SetLocale != nil {
		a.opt.SetLocale(tag)
	}
}

// LocaleChanged follows the language MyGo resolved, either from the user's
// choice or from the system's while following it. The snapshot is rebuilt
// in the background because notes, change logs and the search index are
// written in the language they are built in.
func (a *App) LocaleChanged(tag string) {
	if tag == "" {
		tag = "zh-Hans"
	}
	if tag == a.lang {
		return
	}
	a.lang = tag
	if a.opt.Catalog == nil {
		return
	}
	a.opt.Catalog.SetLocale(tag)
	a.opt.Go(func() {
		a.opt.Catalog.Rebuild()
		a.opt.Post(func() {})
	})
}
