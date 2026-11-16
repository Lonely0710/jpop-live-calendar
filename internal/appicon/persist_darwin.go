package appicon

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"jpop-live-calendar/resources/appicons"
)

// bundlePath is the .app the running executable sits in:
// Live Calendar.app/Contents/MacOS/<exe>.
func bundlePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	app := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
	if !strings.EqualFold(filepath.Ext(app), ".app") || filepath.Base(filepath.Dir(exe)) != "MacOS" {
		return "", ErrNotInstalled
	}
	return app, nil
}

// hasCustomIcon reports whether the bundle carries a custom Finder icon:
// NSWorkspace keeps it in the file "Icon\r" at the root of the bundle.
func hasCustomIcon(app string) bool {
	_, err := os.Stat(filepath.Join(app, "Icon\r"))
	return err == nil
}

func persist(key string, done func(error)) {
	app, err := bundlePath()
	if err != nil {
		done(err)
		return
	}
	var png []byte // nil: back to the bundle's own icon
	if key != appicons.Default {
		png = appicons.PNG(key)
	}
	done(setFileIcon(app, png))
}

func reconcile(key string, done func(error)) {
	app, err := bundlePath()
	if err != nil {
		done(err)
		return
	}
	if hasCustomIcon(app) {
		done(nil) // still there: no update since it was written
		return
	}
	done(setFileIcon(app, appicons.PNG(key)))
}

var (
	selAlloc           = objc.RegisterName("alloc")
	selRelease         = objc.RegisterName("release")
	selDataWithBytes   = objc.RegisterName("dataWithBytes:length:")
	selInitWithData    = objc.RegisterName("initWithData:")
	selStringWithUTF8  = objc.RegisterName("stringWithUTF8String:")
	selSharedWorkspace = objc.RegisterName("sharedWorkspace")
	selSetIcon         = objc.RegisterName("setIcon:forFile:options:")
)

// setFileIcon sets the custom Finder icon of path with
// -[NSWorkspace setIcon:forFile:options:]; nil png removes it. It fails
// when the bundle cannot be written (an app on a read-only disk image,
// or in a folder the user may not write).
func setFileIcon(path string, png []byte) error {
	var img objc.ID
	if len(png) > 0 {
		data := objc.ID(objc.GetClass("NSData")).Send(selDataWithBytes, unsafe.Pointer(&png[0]), uint64(len(png)))
		img = objc.ID(objc.GetClass("NSImage")).Send(selAlloc).Send(selInitWithData, data)
		if img == 0 {
			return errors.New("appicon: the icon is not an image")
		}
		defer img.Send(selRelease)
	}
	cpath := append([]byte(path), 0)
	str := objc.ID(objc.GetClass("NSString")).Send(selStringWithUTF8, unsafe.Pointer(&cpath[0]))
	ws := objc.ID(objc.GetClass("NSWorkspace")).Send(selSharedWorkspace)
	if !objc.Send[bool](ws, selSetIcon, img, str, uint64(0)) {
		return fmt.Errorf("appicon: cannot set the icon of %s", path)
	}
	return nil
}
