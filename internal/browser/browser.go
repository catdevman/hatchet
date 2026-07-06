// Package browser acquires a Chrome/Chromium instance and hands out tabs.
//
// Acquisition paths (HLD §5): an explicit path, discovery of a system
// browser, or a remote CDP endpoint. The managed headless-shell download is a
// milestone-3 addition.
package browser

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/chromedp/chromedp"
)

type Options struct {
	// ChromePath is an explicit browser binary; empty means discover one.
	ChromePath string
	// WSEndpoint connects to an already-running browser over CDP instead of
	// launching one. Takes precedence over ChromePath.
	WSEndpoint string
	// NoSandbox disables Chrome's sandbox, needed in most containers.
	NoSandbox bool
	// Logf receives chromedp debug output; nil means silent.
	Logf func(format string, args ...any)
}

// Browser owns one browser process (or remote connection). All tabs created
// via NewTab share it.
type Browser struct {
	browserCtx context.Context
	cancels    []context.CancelFunc
}

var discoverNames = []string{
	"google-chrome-stable",
	"google-chrome",
	"chromium",
	"chromium-browser",
	"chrome",
	"chrome-headless-shell",
	"headless_shell",
}

var discoverPaths = []string{
	"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	"/Applications/Chromium.app/Contents/MacOS/Chromium",
}

// Discover locates a browser: system Chrome/Chromium first, then hatchet's
// managed headless shell.
func Discover() (string, error) {
	for _, name := range discoverNames {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	for _, p := range discoverPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if p, ok := InstalledShell(); ok {
		return p, nil
	}
	return "", errors.New("no Chrome or Chromium found: run 'hatchet browser install', pass --chrome-path, or connect to a running browser with --browser-ws-endpoint")
}

// New launches (or connects to) a browser. Callers must Close it.
func New(ctx context.Context, opts Options) (*Browser, error) {
	b := &Browser{}

	var allocCtx context.Context
	if opts.WSEndpoint != "" {
		var cancel context.CancelFunc
		allocCtx, cancel = chromedp.NewRemoteAllocator(ctx, opts.WSEndpoint)
		b.cancels = append(b.cancels, cancel)
	} else {
		path := opts.ChromePath
		if path == "" {
			var err error
			if path, err = Discover(); err != nil {
				return nil, err
			}
		}
		execOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
		execOpts = append(execOpts, chromedp.ExecPath(path))
		if opts.NoSandbox {
			execOpts = append(execOpts, chromedp.NoSandbox)
		}
		var cancel context.CancelFunc
		allocCtx, cancel = chromedp.NewExecAllocator(ctx, execOpts...)
		b.cancels = append(b.cancels, cancel)
	}

	var ctxOpts []chromedp.ContextOption
	if opts.Logf != nil {
		ctxOpts = append(ctxOpts, chromedp.WithLogf(opts.Logf), chromedp.WithErrorf(opts.Logf))
	}
	browserCtx, cancel := chromedp.NewContext(allocCtx, ctxOpts...)
	b.cancels = append(b.cancels, cancel)

	// Start the browser now so acquisition errors surface here, not on the
	// first target.
	if err := chromedp.Run(browserCtx); err != nil {
		b.Close()
		return nil, err
	}
	b.browserCtx = browserCtx
	return b, nil
}

// NewTab returns a fresh tab in the shared browser. Cancel closes the tab.
func (b *Browser) NewTab() (context.Context, context.CancelFunc) {
	return chromedp.NewContext(b.browserCtx)
}

func (b *Browser) Close() {
	for i := len(b.cancels) - 1; i >= 0; i-- {
		b.cancels[i]()
	}
}
