// Package vncdeps checks a VNC run's dependencies outside the desktop stack
// itself — the playwright chromium the desktop's web-browser serves.
package vncdeps

import (
	"fmt"
	"path/filepath"
)

// The playwright browser install, as the web-browser wrapper resolves it: the
// env-overridable shared dir holding `playwright install`'s chromium.
const (
	// BrowsersPathEnv is the shared browsers dir's ENV VAR
	BrowsersPathEnv = "PLAYWRIGHT_BROWSERS_PATH"

	// DefaultBrowsersPath is BrowsersPathEnv's fallback — the image's shared copy
	DefaultBrowsersPath = "/opt/ms-playwright"

	// chromeGlob finds a playwright-managed chromium, as the web-browser wrapper does
	chromeGlob = "*/*/chrome"
)

// CheckPlaywright errors when the run serves VNC without a playwright chromium:
// the desktop's web-browser is the session's only browser, so a VNC image
// missing one is broken and refuses to start.
func CheckPlaywright(getenv func(string) string, glob func(string) ([]string, error)) error {
	dir := getenv(BrowsersPathEnv)
	if dir == "" {
		dir = DefaultBrowsersPath
	}
	chromes, err := glob(filepath.Join(dir, chromeGlob))
	if err != nil {
		return fmt.Errorf("vnc requested: find playwright chromium: %w", err)
	}
	if len(chromes) == 0 {
		return fmt.Errorf("vnc requested: no playwright chromium under %s — run: playwright install chromium", dir)
	}
	return nil
}
