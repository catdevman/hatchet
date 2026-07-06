// Command hatchet checks web pages for accessibility issues. It is a thin
// wrapper over pkg/hatchet: flag parsing, config merging, reporting, and the
// exit-code contract (0 clean, 2 issues over threshold, 1 operational error).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/catdevman/hatchet/internal/baseline"
	"github.com/catdevman/hatchet/internal/browser"
	"github.com/catdevman/hatchet/internal/config"
	"github.com/catdevman/hatchet/internal/reporter"
	"github.com/catdevman/hatchet/internal/sitemap"
	"github.com/catdevman/hatchet/pkg/hatchet"
)

// version is set by goreleaser via ldflags.
var version = "dev"

const (
	exitClean       = 0
	exitOperational = 1
	exitIssues      = 2
)

// flagAliases maps short flag names to their canonical long form so
// explicit-set tracking treats -s and --standard as one flag.
var flagAliases = map[string]string{
	"s": "standard",
	"r": "reporter",
	"t": "timeout",
	"w": "wait",
	"T": "threshold",
	"E": "ignore",
	"a": "actions",
	"c": "config",
	"d": "debug",
}

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// settings holds every CLI-configurable value; flags bind into it, config
// defaults fill in what the user didn't set, and per-URL config entries
// override copies of it.
type settings struct {
	standard        string
	reporters       stringList
	timeout         int // ms
	wait            int // ms
	threshold       int
	level           string
	ignore          stringList
	includeNotices  bool
	includeWarnings bool
	rootElement     string
	hideElements    string
	actions         stringList
	viewport        string // "WxH"
	userAgent       string
	cookies         stringList // name=value
	headers         stringList // "Name: value"
	basicAuth       string
	screenCapture   string
	concurrency     int
	sitemap         string
	sitemapFind     string
	sitemapReplace  string
	sitemapExclude  string
	baseline        string
	baselineWrite   string
	baselineUpdate  bool
	baselineRatchet bool
	renderer        string
	axePath         string
	configPath      string
	chromePath      string
	wsEndpoint      string
	noSandbox       bool
	debug           bool
	showVersion     bool
}

func bindFlags(fs *flag.FlagSet, s *settings) {
	fs.StringVar(&s.standard, "standard", "", "accessibility standard: WCAG2A, WCAG2AA (default), WCAG22AA, WCAG2AAA")
	fs.StringVar(&s.standard, "s", "", "alias for --standard")
	fs.Var(&s.reporters, "reporter", "reporter, repeatable: cli, json, csv, sarif, junit, or name=path (default cli)")
	fs.Var(&s.reporters, "r", "alias for --reporter")
	fs.IntVar(&s.timeout, "timeout", 30000, "per-target timeout in milliseconds")
	fs.IntVar(&s.timeout, "t", 30000, "alias for --timeout")
	fs.IntVar(&s.wait, "wait", 0, "extra milliseconds to wait after page load")
	fs.IntVar(&s.wait, "w", 0, "alias for --wait")
	fs.IntVar(&s.threshold, "threshold", 0, "issues at/above --level a target tolerates before failing")
	fs.IntVar(&s.threshold, "T", 0, "alias for --threshold")
	fs.StringVar(&s.level, "level", "error", "minimum issue type that affects the exit code: error, warning, notice")
	fs.Var(&s.ignore, "ignore", "rule code or issue type to ignore, repeatable")
	fs.Var(&s.ignore, "E", "alias for --ignore")
	fs.BoolVar(&s.includeNotices, "include-notices", false, "include notice-level issues")
	fs.BoolVar(&s.includeWarnings, "include-warnings", false, "include warning-level issues")
	fs.StringVar(&s.rootElement, "root-element", "", "CSS selector: only check within this element")
	fs.StringVar(&s.hideElements, "hide-elements", "", "comma-separated CSS selectors to exclude from checking")
	fs.Var(&s.actions, "actions", "pa11y action string to run before checking, repeatable")
	fs.Var(&s.actions, "a", "alias for --actions")
	fs.StringVar(&s.viewport, "viewport", "", "viewport size as WIDTHxHEIGHT (default 1280x1024)")
	fs.StringVar(&s.userAgent, "user-agent", "", "override the browser user agent")
	fs.Var(&s.cookies, "add-cookie", "cookie as name=value, repeatable")
	fs.Var(&s.headers, "add-header", "extra HTTP header as 'Name: value', repeatable")
	fs.StringVar(&s.basicAuth, "basic-auth", "", "HTTP basic auth as user:pass")
	fs.StringVar(&s.screenCapture, "screen-capture", "", "write a full-page PNG to this path (single target only)")
	fs.IntVar(&s.concurrency, "concurrency", 4, "how many targets to check in parallel")
	fs.StringVar(&s.sitemap, "sitemap", "", "load targets from an XML sitemap URL")
	fs.StringVar(&s.sitemapFind, "sitemap-find", "", "string to replace in sitemap URLs (with --sitemap-replace)")
	fs.StringVar(&s.sitemapReplace, "sitemap-replace", "", "replacement for --sitemap-find")
	fs.StringVar(&s.sitemapExclude, "sitemap-exclude", "", "regexp of sitemap URLs to skip")
	fs.StringVar(&s.baseline, "baseline", "", "baseline file: fail only on issues not recorded in it")
	fs.StringVar(&s.baselineWrite, "baseline-write", "", "write current issues to this baseline file and exit 0")
	fs.BoolVar(&s.baselineUpdate, "baseline-update", false, "with --baseline: prune entries for fixed issues")
	fs.BoolVar(&s.baselineRatchet, "baseline-ratchet", false, "with --baseline: fail unless the baselined count decreased")
	fs.StringVar(&s.renderer, "renderer", "", "chrome (default; pages run their JS) or static (page JS disabled, served markup checked as-is)")
	fs.StringVar(&s.axePath, "axe-path", "", "path to an alternate axe-core build (e.g. a locale build)")
	fs.StringVar(&s.configPath, "config", "", "path to JSON config file (default: discover .hatchetrc or hatchet.json)")
	fs.StringVar(&s.configPath, "c", "", "alias for --config")
	fs.StringVar(&s.chromePath, "chrome-path", "", "path to a Chrome/Chromium binary (default: discover)")
	fs.StringVar(&s.wsEndpoint, "browser-ws-endpoint", "", "CDP websocket endpoint of a running browser (launches nothing locally)")
	fs.BoolVar(&s.noSandbox, "no-sandbox", false, "disable Chrome's sandbox (needed in most containers)")
	fs.BoolVar(&s.debug, "debug", false, "log debug output to stderr")
	fs.BoolVar(&s.debug, "d", false, "alias for --debug")
	fs.BoolVar(&s.showVersion, "version", false, "print version and exit")
}

// applySettings lays config values onto s. With a non-nil explicit map, only
// flags the user didn't set are filled (config defaults); with nil, every set
// config field wins (per-URL overrides).
func applySettings(cfg config.Settings, explicit map[string]bool, s *settings) {
	unset := func(name string) bool { return explicit == nil || !explicit[name] }

	if unset("standard") && cfg.Standard != "" {
		s.standard = cfg.Standard
	}
	if unset("timeout") && cfg.Timeout != nil {
		s.timeout = *cfg.Timeout
	}
	if unset("wait") && cfg.Wait != nil {
		s.wait = *cfg.Wait
	}
	if unset("threshold") && cfg.Threshold != nil {
		s.threshold = *cfg.Threshold
	}
	if unset("level") && cfg.Level != "" {
		s.level = cfg.Level
	}
	if unset("reporter") && len(cfg.Reporters) > 0 {
		s.reporters = cfg.Reporters
	}
	if unset("ignore") && len(cfg.Ignore) > 0 {
		s.ignore = cfg.Ignore
	}
	if unset("include-notices") && cfg.IncludeNotices != nil {
		s.includeNotices = *cfg.IncludeNotices
	}
	if unset("include-warnings") && cfg.IncludeWarnings != nil {
		s.includeWarnings = *cfg.IncludeWarnings
	}
	if unset("root-element") && cfg.RootElement != "" {
		s.rootElement = cfg.RootElement
	}
	if unset("hide-elements") && cfg.HideElements != "" {
		s.hideElements = cfg.HideElements
	}
	if unset("actions") && len(cfg.Actions) > 0 {
		s.actions = cfg.Actions
	}
	if unset("renderer") && cfg.Renderer != "" {
		s.renderer = cfg.Renderer
	}
	if unset("axe-path") && cfg.AxePath != "" {
		s.axePath = cfg.AxePath
	}
	if unset("viewport") && cfg.Viewport != nil {
		s.viewport = fmt.Sprintf("%dx%d", cfg.Viewport.Width, cfg.Viewport.Height)
	}
	if unset("user-agent") && cfg.UserAgent != "" {
		s.userAgent = cfg.UserAgent
	}
	if unset("add-header") && len(cfg.Headers) > 0 {
		s.headers = nil
		for k, v := range cfg.Headers {
			s.headers = append(s.headers, k+": "+v)
		}
	}
	if unset("concurrency") && cfg.Concurrency != nil {
		s.concurrency = *cfg.Concurrency
	}
	if unset("chrome-path") && cfg.ChromePath != "" {
		s.chromePath = cfg.ChromePath
	}
	if unset("browser-ws-endpoint") && cfg.BrowserWSEndpoint != "" {
		s.wsEndpoint = cfg.BrowserWSEndpoint
	}
	if unset("no-sandbox") && cfg.NoSandbox != nil {
		s.noSandbox = *cfg.NoSandbox
	}
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "browser" {
		os.Exit(runBrowser(args[1:]))
	}
	os.Exit(run(args, os.Stdin))
}

func runBrowser(args []string) int {
	if len(args) != 1 || args[0] != "install" {
		fmt.Fprintln(os.Stderr, "Usage: hatchet browser install")
		return exitOperational
	}
	logf := log.New(os.Stderr, "hatchet: ", 0).Printf
	if _, err := browser.Install(context.Background(), logf); err != nil {
		return fail(err)
	}
	return exitClean
}

func run(args []string, stdin io.Reader) int {
	fs := flag.NewFlagSet("hatchet", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), `Usage: hatchet [options] <url | path | -> [<url>...]
       hatchet browser install

Options:
`)
		fs.PrintDefaults()
	}
	var s settings
	bindFlags(fs, &s)

	if err := fs.Parse(args); err != nil {
		return exitOperational
	}

	if s.showVersion {
		fmt.Printf("hatchet %s (axe-core %s, chrome-headless-shell pin %s)\n",
			version, hatchet.AxeVersion(), browser.ShellVersion)
		return exitClean
	}

	explicit := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		name := f.Name
		if canonical, ok := flagAliases[name]; ok {
			name = canonical
		}
		explicit[name] = true
	})

	cfg, err := loadConfig(s.configPath)
	if err != nil {
		return fail(err)
	}
	applySettings(cfg.EffectiveDefaults(), explicit, &s)

	levelCode, err := levelTypeCode(s.level)
	if err != nil {
		return fail(err)
	}

	targets, thresholds, err := resolveTargets(fs.Args(), &s, cfg, stdin)
	if err != nil {
		return fail(err)
	}
	if s.screenCapture != "" && len(targets) > 1 {
		return fail(fmt.Errorf("--screen-capture works with a single target, got %d", len(targets)))
	}

	opts, err := buildOptions(&s)
	if err != nil {
		return fail(err)
	}

	results, err := hatchet.RunTargets(context.Background(), targets, opts)
	if err != nil {
		return fail(err)
	}

	if s.baselineWrite != "" {
		bl := baseline.New(results, hatchet.AxeVersion())
		if err := bl.Write(s.baselineWrite); err != nil {
			return fail(err)
		}
		fmt.Fprintf(os.Stderr, "hatchet: wrote baseline with %d entries to %s\n", len(bl.Entries), s.baselineWrite)
		if err := reportAll(&s, results); err != nil {
			return fail(err)
		}
		return exitClean
	}

	var ratchetStuck bool
	if s.baseline != "" {
		stuck, err := applyBaseline(&s, results)
		if err != nil {
			return fail(err)
		}
		ratchetStuck = stuck
	}

	if err := reportAll(&s, results); err != nil {
		return fail(err)
	}

	code := exitCode(results, levelCode, thresholds)
	if code == exitClean && ratchetStuck {
		fmt.Fprintln(os.Stderr, "hatchet: baseline ratchet: no baselined issues were fixed")
		return exitIssues
	}
	return code
}

// resolveTargets builds the target list (positional args, sitemap, or config
// urls) with a per-target threshold aligned to it.
func resolveTargets(positional []string, s *settings, cfg *config.File, stdin io.Reader) ([]hatchet.Target, []int, error) {
	switch {
	case s.sitemap != "":
		if len(positional) > 0 {
			return nil, nil, fmt.Errorf("--sitemap and positional targets are mutually exclusive")
		}
		urls, err := sitemapTargets(s)
		if err != nil {
			return nil, nil, err
		}
		return plainTargets(urls, s.threshold)

	case len(positional) > 0:
		expanded, err := expandStdin(positional, stdin)
		if err != nil {
			return nil, nil, err
		}
		return plainTargets(expanded, s.threshold)

	case len(cfg.URLs) > 0:
		targets := make([]hatchet.Target, 0, len(cfg.URLs))
		thresholds := make([]int, 0, len(cfg.URLs))
		for _, entry := range cfg.URLs {
			merged := *s
			applySettings(entry.Settings, nil, &merged)
			opts, err := buildOptions(&merged)
			if err != nil {
				return nil, nil, fmt.Errorf("config url %s: %w", entry.URL, err)
			}
			targets = append(targets, hatchet.Target{URL: entry.URL, Options: &opts})
			thresholds = append(thresholds, merged.threshold)
		}
		return targets, thresholds, nil

	default:
		return nil, nil, fmt.Errorf("no targets: pass URLs, --sitemap, or a config file with \"urls\"")
	}
}

func plainTargets(urls []string, threshold int) ([]hatchet.Target, []int, error) {
	targets := make([]hatchet.Target, len(urls))
	thresholds := make([]int, len(urls))
	for i, u := range urls {
		targets[i] = hatchet.Target{URL: u}
		thresholds[i] = threshold
	}
	return targets, thresholds, nil
}

func sitemapTargets(s *settings) ([]string, error) {
	urls, err := sitemap.Fetch(context.Background(), s.sitemap, nil)
	if err != nil {
		return nil, err
	}
	var exclude *regexp.Regexp
	if s.sitemapExclude != "" {
		if exclude, err = regexp.Compile(s.sitemapExclude); err != nil {
			return nil, fmt.Errorf("invalid --sitemap-exclude: %w", err)
		}
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if exclude != nil && exclude.MatchString(u) {
			continue
		}
		if s.sitemapFind != "" {
			u = strings.ReplaceAll(u, s.sitemapFind, s.sitemapReplace)
		}
		out = append(out, u)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("sitemap %s yielded no targets after filtering", s.sitemap)
	}
	return out, nil
}

// applyBaseline filters known issues out of results. It returns whether the
// ratchet check failed (nothing fixed).
func applyBaseline(s *settings, results []hatchet.Result) (bool, error) {
	bl, err := baseline.Load(s.baseline)
	if err != nil {
		return false, err
	}
	if bl.AxeVersion != hatchet.AxeVersion() {
		fmt.Fprintf(os.Stderr, "hatchet: warning: baseline was written with axe-core %s, this binary embeds %s; results may shift\n",
			bl.AxeVersion, hatchet.AxeVersion())
	}

	origCount := len(bl.Entries)
	if s.baselineUpdate {
		bl.Prune(results)
		if err := bl.Write(s.baseline); err != nil {
			return false, err
		}
	}
	stats := bl.Apply(results)
	if s.baselineUpdate {
		stats.Fixed = origCount - len(bl.Entries)
	}
	fmt.Fprintf(os.Stderr, "hatchet: baseline: %d new, %d baselined, %d fixed\n",
		stats.New, stats.Baselined, stats.Fixed)

	stuck := s.baselineRatchet && origCount > 0 && stats.Fixed == 0
	return stuck, nil
}

func fail(err error) int {
	fmt.Fprintf(os.Stderr, "hatchet: %v\n", err)
	return exitOperational
}

func buildOptions(s *settings) (hatchet.Options, error) {
	opts := hatchet.Options{
		Standard:          s.standard,
		Timeout:           time.Duration(s.timeout) * time.Millisecond,
		Wait:              time.Duration(s.wait) * time.Millisecond,
		IncludeNotices:    s.includeNotices,
		IncludeWarnings:   s.includeWarnings,
		Ignore:            s.ignore,
		RootElement:       s.rootElement,
		HideElements:      s.hideElements,
		Actions:           s.actions,
		Renderer:          s.renderer,
		AxePath:           s.axePath,
		UserAgent:         s.userAgent,
		BasicAuth:         s.basicAuth,
		ScreenCapture:     s.screenCapture,
		Concurrency:       s.concurrency,
		ChromePath:        s.chromePath,
		BrowserWSEndpoint: s.wsEndpoint,
		NoSandbox:         s.noSandbox,
	}
	if s.debug {
		opts.Logf = log.New(os.Stderr, "hatchet: ", log.LstdFlags).Printf
	}

	if s.viewport != "" {
		w, h, err := parseViewport(s.viewport)
		if err != nil {
			return opts, err
		}
		opts.ViewportWidth, opts.ViewportHeight = w, h
	}

	for _, c := range s.cookies {
		name, value, ok := strings.Cut(c, "=")
		if !ok || name == "" {
			return opts, fmt.Errorf("invalid cookie %q (expected name=value)", c)
		}
		opts.Cookies = append(opts.Cookies, hatchet.Cookie{Name: name, Value: value})
	}

	if len(s.headers) > 0 {
		opts.Headers = make(map[string]string, len(s.headers))
		for _, h := range s.headers {
			name, value, ok := strings.Cut(h, ":")
			if !ok || strings.TrimSpace(name) == "" {
				return opts, fmt.Errorf("invalid header %q (expected 'Name: value')", h)
			}
			opts.Headers[strings.TrimSpace(name)] = strings.TrimSpace(value)
		}
	}
	return opts, nil
}

func parseViewport(v string) (int, int, error) {
	ws, hs, ok := strings.Cut(v, "x")
	if ok {
		w, werr := strconv.Atoi(ws)
		h, herr := strconv.Atoi(hs)
		if werr == nil && herr == nil && w > 0 && h > 0 {
			return w, h, nil
		}
	}
	return 0, 0, fmt.Errorf("invalid viewport %q (expected WIDTHxHEIGHT, e.g. 1280x1024)", v)
}

// expandStdin replaces a "-" target with a temp file holding stdin's HTML.
func expandStdin(targets []string, stdin io.Reader) ([]string, error) {
	out := make([]string, len(targets))
	var stdinPath string
	for i, t := range targets {
		if t != "-" {
			out[i] = t
			continue
		}
		if stdinPath == "" {
			html, err := io.ReadAll(stdin)
			if err != nil {
				return nil, fmt.Errorf("reading stdin: %w", err)
			}
			f, err := os.CreateTemp("", "hatchet-stdin-*.html")
			if err != nil {
				return nil, err
			}
			if _, err := f.Write(html); err != nil {
				f.Close()
				return nil, err
			}
			if err := f.Close(); err != nil {
				return nil, err
			}
			stdinPath = f.Name()
		}
		out[i] = stdinPath
	}
	return out, nil
}

func loadConfig(path string) (*config.File, error) {
	if path != "" {
		return config.Load(path)
	}
	return config.Discover(".")
}

func levelTypeCode(level string) (int, error) {
	switch level {
	case hatchet.TypeError:
		return 1, nil
	case hatchet.TypeWarning:
		return 2, nil
	case hatchet.TypeNotice:
		return 3, nil
	default:
		return 0, fmt.Errorf("unknown level %q (expected error, warning, or notice)", level)
	}
}

func reportAll(s *settings, results []hatchet.Result) error {
	specs := s.reporters
	if len(specs) == 0 {
		specs = stringList{"cli"}
	}
	return report(specs, results)
}

// report runs every reporter spec ("name" or "name=path").
func report(specs []string, results []hatchet.Result) error {
	for _, spec := range specs {
		name, path, _ := strings.Cut(spec, "=")

		var w io.Writer = os.Stdout
		if path != "" {
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			defer f.Close()
			w = f
		}

		var r reporter.Reporter
		switch name {
		case "cli":
			r = reporter.CLI{Color: useColor(w)}
		case "json":
			r = reporter.JSON{HatchetVersion: version, AxeVersion: hatchet.AxeVersion()}
		case "csv":
			r = reporter.CSV{}
		case "sarif":
			r = reporter.SARIF{HatchetVersion: version}
		case "junit":
			r = reporter.JUnit{}
		default:
			return fmt.Errorf("unknown reporter %q (expected cli, json, csv, sarif, or junit)", name)
		}
		if err := r.Report(w, results); err != nil {
			return err
		}
	}
	return nil
}

func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// exitCode fails when any target's issue count (at/above the level) exceeds
// that target's threshold. thresholds is aligned with results.
func exitCode(results []hatchet.Result, levelCode int, thresholds []int) int {
	failed := false
	operational := false
	for i, r := range results {
		if r.Err != nil {
			operational = true
			continue
		}
		count := 0
		for _, is := range r.Issues {
			if is.TypeCode <= levelCode {
				count++
			}
		}
		threshold := 0
		if i < len(thresholds) {
			threshold = thresholds[i]
		}
		if count > threshold {
			failed = true
		}
	}
	if operational {
		return exitOperational
	}
	if failed {
		return exitIssues
	}
	return exitClean
}
