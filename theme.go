package main

// Light, dark or following the system. The choice is stored in the settings
// table (ui_theme), read before the window opens so it starts in the right
// colors, and applied to the native window too (the macOS title bar area).

import (
	"context"
	"errors"
	"os/exec"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"googleplex/internal/db"
)

func validTheme(t string) string {
	if t == "light" || t == "dark" {
		return t
	}
	return "system"
}

// storedTheme reads the saved choice without the app running yet.
func storedTheme() string {
	path, err := db.Path()
	if err != nil {
		return "system"
	}
	conn, err := db.Open(path)
	if err != nil {
		return "system"
	}
	defer conn.Close()
	return validTheme(db.GetSetting(conn, "ui_theme"))
}

// systemIsDark guesses the OS setting before a window exists (macOS only;
// elsewhere the app starts dark, as it always has).
func systemIsDark() bool {
	if runtime.GOOS != "darwin" {
		return true
	}
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	return err == nil && strings.TrimSpace(string(out)) == "Dark"
}

func isDark(theme string) bool {
	return theme == "dark" || (theme == "system" && systemIsDark())
}

// windowColour matches the page background, so nothing flashes before it loads.
func windowColour(theme string) *options.RGBA {
	if isDark(theme) {
		return &options.RGBA{R: 12, G: 13, B: 16, A: 1}
	}
	return &options.RGBA{R: 248, G: 249, B: 250, A: 1}
}

func macAppearance(theme string) mac.AppearanceType {
	switch theme {
	case "light":
		return mac.NSAppearanceNameAqua
	case "dark":
		return mac.NSAppearanceNameDarkAqua
	}
	return mac.DefaultAppearance
}

// GetTheme is "system", "light" or "dark".
func (a *App) GetTheme() string { return validTheme(db.GetSetting(a.db, "ui_theme")) }

// SetTheme saves the choice and applies it to the native window; the page
// switches its own colors, and the pop-out picks it up from /pip/theme.
func (a *App) SetTheme(theme string) error {
	if validTheme(theme) != theme {
		return errors.New("theme is system, light or dark")
	}
	if err := db.SetSetting(a.db, "ui_theme", theme); err != nil {
		return err
	}
	applyNativeTheme(a.ctx, theme)
	return nil
}

// applyNativeTheme sets the window's own appearance: NSAppearance on macOS,
// the title bar theme on Windows.
func applyNativeTheme(ctx context.Context, theme string) {
	setAppAppearance(theme)
	switch theme {
	case "light":
		wruntime.WindowSetLightTheme(ctx)
	case "dark":
		wruntime.WindowSetDarkTheme(ctx)
	default:
		wruntime.WindowSetSystemDefaultTheme(ctx)
	}
}
