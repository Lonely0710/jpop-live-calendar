// Live Calendar — 日本音乐 Live 情报日历（macOS / Windows 原生，MyGo + Go）。
package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"jpop-live-calendar/config"
	"jpop-live-calendar/internal/avatar"
	"jpop-live-calendar/internal/cache"
	"jpop-live-calendar/internal/domain"
	"jpop-live-calendar/internal/theme"
	"jpop-live-calendar/internal/views"
)

const version = "1.0.2"

// trayIcon is the black-and-transparent menu bar icon; macOS tints it.
//
//go:embed resources/tray.png
var trayIcon []byte

// trayIconWindows is the coral icon for the Windows notification area,
// where template icons are not tinted.
//
//go:embed resources/tray-win.png
var trayIconWindows []byte

// pacificoFont is the script face of the JPOP wordmark. It is embedded so
// the app shows it even on a Mac without the font installed.
//
//go:embed resources/Pacifico-Regular.ttf
var pacificoFont []byte

func main() {
	cfg, err := config.Load()
	if err != nil {
		// A broken sources.yaml must not stop the app: run on the
		// built-in defaults and say so on the 数据源 page.
		log.Printf("sources.yaml: %v; using the built-in defaults", err)
		cfg = config.Default()
		cfg.Path = config.Embedded + "（" + err.Error() + "）"
	}
	pins, pinErr := config.LoadPins()
	if pinErr != nil {
		log.Printf("pins.json: %v", pinErr)
	}
	cachePath, cacheErr := config.CachePath()
	if cacheErr != nil {
		log.Printf("cache: %v", cacheErr)
	}
	notified, notifiedErr := config.LoadNotified()
	if notifiedErr != nil {
		log.Printf("notified.json: %v", notifiedErr)
		notified = map[string]string{}
	}
	if err := ui.RegisterFont(pacificoFont, "Pacifico"); err != nil {
		log.Printf("Pacifico: %v", err)
	}
	avatarPath, avatarErr := config.AvatarCachePath()
	var avatarStore = avatar.NewStore()
	if avatarErr != nil {
		log.Printf("avatars.json: %v", avatarErr)
		avatarPath = ""
	} else if cached, err := cache.LoadAvatars(avatarPath); err != nil {
		log.Printf("avatars.json: %v", err)
	} else {
		avatarStore.Restore(cached)
	}
	var win *mygo.Window
	app := views.New(views.Options{
		Config:   cfg,
		Version:  version,
		Pinned:   pins,
		SavePins: config.SavePins,
		// 上次同步的公告快照：启动时先显示，之后每次同步完成再写回。
		CachePath: cachePath,
		// 抽选截止提醒：系统通知；同一截止只提醒一次（notified.json）。
		Notified:     notified,
		SaveNotified: config.SaveNotified,
		Notify: func(r views.Reminder) error {
			if !mygo.NotificationsSupported() {
				return errors.New("系统通知不可用（需要打包后的 App）")
			}
			n := mygo.NewNotification(mygo.NotificationOptions{
				ID: r.ID, Title: r.Title, Subtitle: r.Subtitle, Body: r.Body, Group: "deadlines",
			})
			return n.Show()
		},
		// 删除数据源时写回 sources.yaml（内置配置则写到用户配置目录）。
		RemoveSource: func(id string) (string, error) {
			p, err := config.RemoveSource(cfg.Path, id)
			if err == nil {
				cfg.Path = p
			}
			return p, err
		},
		AddSource: func(src domain.SourceConfig) (string, error) {
			p, err := config.AddSource(cfg.Path, src)
			if err == nil {
				cfg.Path = p
			}
			return p, err
		},
		// 艺人头像与主题色：后台从 Deezer 公开 API 获取（avatars: off 可关闭），
		// 公开图片缓存在 avatars.json，启动先用缓存，条件请求只替换有变化的图。
		Avatars:       avatarStore,
		AvatarGetters: avatar.NewGetters(cfg.Avatars),
		SaveAvatars: func(m map[string]avatar.Avatar) error {
			if avatarPath == "" {
				return nil
			}
			return cache.SaveAvatars(avatarPath, m)
		},
		// Results of background syncs come back on the main thread.
		Post: func(fn func()) {
			if win != nil {
				win.Update(fn)
			}
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// A click on a deadline notification brings the window back and opens
	// the deadline's entry. The ID survives a quit, so it may name a
	// deadline of an earlier run too: if the click starts the app, the
	// window comes later and the entry waits in pendingEntry.
	pendingEntry := ""
	mygo.App.OnNotificationClick(func(id string) {
		entry := views.EntryIDFromNotification(id)
		if entry == "" {
			return
		}
		if win == nil {
			pendingEntry = entry
			return
		}
		win.Show()
		win.Focus()
		mygo.App.Focus()
		win.Update(func() { app.OpenEntry(entry) })
	})
	mygo.App.WhenReady(func() {
		opts := mygo.WindowOptions{
			Title:           "Live Calendar",
			Width:           theme.WindowWidth,
			Height:          theme.WindowHeight,
			MinWidth:        theme.MinWidth,
			MinHeight:       theme.MinHeight,
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  theme.TitleBarHeight,
			BackgroundColor: "#F3E9D8",
			StateKey:        "main",
			Content:         ui.View(app.View),
		}
		if runtime.GOOS == "darwin" {
			// The traffic lights centered in the 52-pt top band, on one line
			// with the toolbar: c.TitleBar().Height is then 52 too.
			opts.TitleBarStyle = mygo.TitleBarHiddenInset
			opts.TrafficLightPosition = &mygo.Point{X: theme.TrafficLightX, Y: theme.TrafficLightY}
		}
		win = mygo.NewWindow(opts)
		// Sync every enabled source now, then on each one's interval.
		app.Start(ctx)
		setupMenuBar(app, win)
		if entry := pendingEntry; entry != "" {
			pendingEntry = ""
			win.Show()
			win.Focus()
			mygo.App.Focus()
			win.Update(func() { app.OpenEntry(entry) })
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// tray is the menu bar / notification-area icon; it must stay referenced
// for as long as the application runs.
var tray *mygo.Tray

// setupMenuBar adds the tray icon and keeps the app alive when the window is
// closed: closing hides the window, and the sync, the deadlines and the tray
// stay. The tray's first item brings it back; 退出 is the only way to quit.
func setupMenuBar(app *views.App, win *mygo.Window) {
	quitting := false
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true })
	show := func() {
		win.Show()
		win.Focus()
		mygo.App.Focus()
	}
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开 Live Calendar", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
		{Label: "立即同步", Click: func(*mygo.MenuItem, *mygo.Window) { app.Refresh() }},
		{Type: mygo.MenuItemSeparator},
		{Label: "退出 Live Calendar", Role: mygo.RoleQuit, Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() }},
	})
	icon := trayIcon
	if runtime.GOOS == "windows" {
		icon = trayIconWindows
	}
	t, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           icon,
		IconIsTemplate: runtime.GOOS == "darwin",
		ToolTip:        "Live Calendar · 日本音乐 Live 情报日历",
		Menu:           menu,
	})
	if err != nil {
		log.Printf("menu bar: %v", err)
	}
	tray = t
	win.OnClose(func(e *mygo.CloseEvent) {
		if quitting {
			return
		}
		e.PreventDefault()
		win.Hide()
	})
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			show()
		}
	})
}
