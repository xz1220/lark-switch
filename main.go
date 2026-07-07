// lark-switch — a gvm-style multi-account switcher for the Lark/Feishu CLI (lark-cli).
//
// Each account is pinned to its own config home via LARKSUITE_CLI_CONFIG_DIR, so
// two Feishu accounts (typically in different tenants, each its own self-built app)
// can coexist and be used in parallel without the global-state races of
// `lark-cli profile use`. The default-home account is special-cased: it never gets
// the env var set (see store.go:isDefaultHome).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const version = "0.2.0"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	cmd, rest := args[0], args[1:]

	var err error
	switch cmd {
	case "add":
		err = cmdAdd(rest)
	case "ls", "list":
		err = cmdLs(rest)
	case "use":
		err = cmdUse(rest)
	case "run":
		err = cmdRun(rest)
	case "each":
		err = cmdEach(rest)
	case "login":
		err = cmdLogin(rest)
	case "refresh":
		err = cmdRefresh(rest)
	case "rm", "remove":
		err = cmdRm(rest)
	case "current", "which":
		err = cmdCurrent(rest)
	case "path":
		err = cmdPath(rest)
	case "shellenv":
		err = cmdShellenv(rest)
	case "help", "-h", "--help":
		usage()
	case "version", "-v", "--version":
		fmt.Println("lark-switch", version)
	default:
		fmt.Fprintf(os.Stderr, "lark-switch: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "lark-switch: "+err.Error())
		os.Exit(1)
	}
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	dir := fs.String("dir", "", "config home dir (default: ~/.lark-cli-<name>)")
	brand := fs.String("brand", "feishu", "feishu | lark")
	note := fs.String("note", "", "free-form note")
	doInit := fs.Bool("init", false, "after registering, run config init + auth login (interactive)")
	domain := fs.String("domain", "all", "scope domain(s) for the auth login step")
	name, rest := splitNameFlags(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return fmt.Errorf("usage: lark-switch add <name> [--dir <path>] [--init] [--domain ...]")
	}
	if strings.ContainsAny(name, "/ \t\n") {
		return fmt.Errorf("invalid account name %q", name)
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	if s.find(name) != nil {
		return fmt.Errorf("account %q already exists", name)
	}
	d := *dir
	if d == "" {
		d = "~/.lark-cli-" + name
	}
	abs := expandHome(d)
	if a := s.findByDir(abs); a != nil {
		return fmt.Errorf("dir %s is already registered as %q", abs, a.Name)
	}
	s.Accounts = append(s.Accounts, Account{Name: name, Dir: abs, Brand: *brand, Note: *note})
	if err := s.save(); err != nil {
		return err
	}
	fmt.Printf("registered %q -> %s\n", name, shortDir(abs))
	if isDefaultHome(abs) {
		fmt.Println("  (default home — LARKSUITE_CLI_CONFIG_DIR will never be set for this account)")
	}

	if *doInit {
		a := s.find(name)
		fmt.Printf("\n# setting up %q — a browser / QR-code authorization step is coming up\n", name)
		if err := runInteractive(a, "config", "init", "--new"); err != nil {
			return fmt.Errorf("config init failed: %w", err)
		}
		if err := runInteractive(a, "auth", "login", "--domain", *domain); err != nil {
			return fmt.Errorf("auth login failed: %w", err)
		}
		fmt.Printf("\n✓ %q is ready. Try: lark-switch ls\n", name)
		return nil
	}
	fmt.Printf("to log in later: lark-switch login %s\n", name)
	return nil
}

func cmdLogin(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	domain := fs.String("domain", "all", "scope domain(s)")
	scope := fs.String("scope", "", "specific scope(s) (overrides --domain)")
	initFirst := fs.Bool("init", false, "run config init --new before auth login")
	name, rest := splitNameFlags(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return fmt.Errorf("usage: lark-switch login <name> [--domain ...|--scope ...] [--init]")
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	a := s.find(name)
	if a == nil {
		return fmt.Errorf("no such account %q (see `lark-switch ls`)", name)
	}
	if *initFirst {
		if err := runInteractive(a, "config", "init", "--new"); err != nil {
			return fmt.Errorf("config init failed: %w", err)
		}
	}
	larkArgs := []string{"auth", "login"}
	if *scope != "" {
		larkArgs = append(larkArgs, "--scope", *scope)
	} else {
		larkArgs = append(larkArgs, "--domain", *domain)
	}
	return runInteractive(a, larkArgs...)
}

func cmdUse(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lark-switch use <name>\n(needs the shell shim: eval \"$(lark-switch shellenv)\", then `lk use <name>`)")
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	a := s.find(args[0])
	if a == nil {
		return fmt.Errorf("no such account %q (see `lark-switch ls`)", args[0])
	}
	// Emit shell code on stdout for `eval`. stderr carries the human message.
	if isDefaultHome(a.Dir) {
		fmt.Println("unset LARKSUITE_CLI_CONFIG_DIR")
	} else {
		fmt.Printf("export LARKSUITE_CLI_CONFIG_DIR=%s\n", shellQuote(expandHome(a.Dir)))
	}
	fmt.Printf("export LARK_SWITCH_CURRENT=%s\n", shellQuote(a.Name))
	if os.Getenv("LARK_SWITCH_EVAL") != "" {
		fmt.Fprintf(os.Stderr, "lark-switch: now using %q\n", a.Name)
		return nil
	}
	// Without LARK_SWITCH_EVAL nothing is eval'ing our stdout: the exports above
	// were merely displayed and the parent's environment is unchanged. This is
	// exactly what happens when an agent runs `use` in a tool call — fail loudly
	// with the working alternative instead of pretending the switch happened.
	fmt.Fprintf(os.Stderr, "lark-switch: nothing was switched — `use` only prints exports; a child process cannot change its parent's environment.\n")
	if stdoutIsTTY() {
		fmt.Fprintf(os.Stderr, "  one-off:   eval \"$(LARK_SWITCH_EVAL=1 lark-switch use %s)\"\n", a.Name)
		fmt.Fprintf(os.Stderr, "  permanent: add  eval \"$(lark-switch shellenv)\"  to ~/.zshrc, then `lark-switch use %s` just works\n", a.Name)
	} else {
		fmt.Fprintf(os.Stderr, "  run one command as %s:  lark-switch run %s -- <lark-cli args>\n", a.Name, a.Name)
		if !isDefaultHome(a.Dir) {
			fmt.Fprintf(os.Stderr, "  or pin per command:     LARKSUITE_CLI_CONFIG_DIR=\"$(lark-switch path %s)\" lark-cli <args>\n", a.Name)
		}
	}
	os.Exit(1)
	return nil
}

func cmdRun(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: lark-switch run <name> [--] <lark-cli args...>")
	}
	name := args[0]
	rest := stripDashDash(args[1:])
	if len(rest) == 0 {
		return fmt.Errorf("usage: lark-switch run %s [--] <lark-cli args...>", name)
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	a := s.find(name)
	if a == nil {
		return fmt.Errorf("no such account %q (see `lark-switch ls`)", name)
	}
	larkCli, err := larkCliPath()
	if err != nil {
		return err
	}
	argv := append([]string{larkCli}, rest...)
	// Replace this process so exit code / signals pass through cleanly.
	return syscall.Exec(larkCli, argv, envForAccount(a))
}

func cmdEach(args []string) error {
	rest := stripDashDash(args)
	if len(rest) == 0 {
		return fmt.Errorf("usage: lark-switch each [--] <lark-cli args...>")
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts registered (add one with `lark-switch add <name> --init`)")
	}
	larkCli, err := larkCliPath()
	if err != nil {
		return err
	}
	var failed int
	for i := range s.Accounts {
		a := &s.Accounts[i]
		fmt.Printf("\n==== %s (%s) ====\n", a.Name, shortDir(a.Dir))
		c := exec.Command(larkCli, rest...)
		c.Env = envForAccount(a)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		if err := c.Run(); err != nil {
			failed++
			fmt.Fprintf(os.Stderr, "  (%s failed: %v)\n", a.Name, err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d accounts failed", failed, len(s.Accounts))
	}
	return nil
}

func cmdRefresh(args []string) error {
	fs := flag.NewFlagSet("refresh", flag.ContinueOnError)
	all := fs.Bool("all", false, "refresh every registered account")
	name, rest := splitNameFlags(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	larkCli, err := larkCliPath()
	if err != nil {
		return err
	}
	var targets []*Account
	if *all || name == "" {
		for i := range s.Accounts {
			targets = append(targets, &s.Accounts[i])
		}
	} else {
		a := s.find(name)
		if a == nil {
			return fmt.Errorf("no such account %q", name)
		}
		targets = append(targets, a)
	}
	if len(targets) == 0 {
		return fmt.Errorf("no accounts registered")
	}
	var failed int
	for _, a := range targets {
		if err := keepAlive(larkCli, a); err != nil {
			failed++
			fmt.Printf("✗ %-12s %v\n", a.Name, err)
			continue
		}
		exp := "?"
		if st, e := queryStatus(larkCli, a); e == nil {
			if d, ok := refreshRemaining(st); ok {
				exp = humanDur(d)
			}
		}
		fmt.Printf("✓ %-12s ok (refresh window now ~%s)\n", a.Name, exp)
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d accounts failed", failed, len(targets))
	}
	return nil
}

// lsRow is one account's full state — rendered either as a table row or, with
// --json, marshaled directly (lowercase fields stay table-only).
type lsRow struct {
	Name             string `json:"name"`
	Dir              string `json:"dir"`
	Current          bool   `json:"current"`
	DefaultHome      bool   `json:"default_home"`
	Brand            string `json:"brand,omitempty"`
	Note             string `json:"note,omitempty"`
	User             string `json:"user,omitempty"`
	OpenID           string `json:"open_id,omitempty"`
	Status           string `json:"status"`
	RefreshExpiresAt string `json:"refresh_expires_at,omitempty"`
	RefreshInSeconds *int64 `json:"refresh_in_seconds,omitempty"`

	configured bool
	refresh    string
	warn       bool
}

func cmdLs(args []string) error {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	if len(s.Accounts) == 0 {
		if *jsonOut {
			fmt.Println(`{"accounts": []}`)
			return nil
		}
		fmt.Println("no accounts registered yet.\n\nregister the existing default account, then add the second:")
		fmt.Println("  lark-switch add A --dir ~/.lark-cli      # already-configured default")
		fmt.Println("  lark-switch add B --init                 # new home + login")
		return nil
	}
	larkCli, _ := larkCliPath()
	cur := currentName(s)

	rows := make([]lsRow, 0, len(s.Accounts))
	for i := range s.Accounts {
		a := &s.Accounts[i]
		r := lsRow{
			Name: a.Name, Dir: expandHome(a.Dir), Current: a.Name == cur,
			DefaultHome: isDefaultHome(a.Dir), Brand: a.Brand, Note: a.Note,
			Status: "unknown", refresh: "-",
		}
		if larkCli == "" {
			r.Status = "lark-cli-not-found"
		} else {
			st, err := queryStatus(larkCli, a)
			switch {
			case err != nil:
				r.Status = "error"
			case !st.configured():
				r.Status = "not-configured"
			default:
				r.configured = true
				r.User = st.Identities.User.UserName
				r.OpenID = st.Identities.User.OpenID
				r.Status = st.Identities.User.TokenStatus
				if r.Status == "" {
					r.Status = "unknown"
				}
				r.RefreshExpiresAt = st.Identities.User.RefreshExpiresAt
				if d, ok := refreshRemaining(st); ok {
					sec := int64(d.Seconds())
					r.RefreshInSeconds = &sec
					r.refresh = humanDur(d)
					r.warn = d < 48*time.Hour
				}
			}
		}
		rows = append(rows, r)
	}

	if *jsonOut {
		out := struct {
			Current  string  `json:"current,omitempty"`
			Accounts []lsRow `json:"accounts"`
		}{cur, rows}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	color := useColor()
	fmt.Printf("%-2s %s %s %s %s %s\n", "", cell("NAME", 10), cell("USER", 14), cell("STATUS", 14), cell("REFRESH-IN", 11), "DIR")
	for _, r := range rows {
		mark := " "
		if r.Current {
			mark = "*"
		}
		name := cell(r.Name, 10)
		if color && r.Current {
			name = bold(name)
		}
		user := r.User
		if user == "" {
			user = "-"
			if r.configured {
				user = "(bot only)"
			}
		}
		status := r.Status
		if status == "unknown" {
			status = "-"
		}
		refresh := cell(r.refresh, 11)
		if color && r.warn {
			refresh = red(refresh)
		}
		fmt.Printf("%-2s %s %s %s %s %s\n", mark, name, cell(user, 14), cell(status, 14), refresh, shortDir(r.Dir))
	}
	return nil
}

func cmdRm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	purge := fs.Bool("purge", false, "also delete the account's config home directory")
	name, rest := splitNameFlags(args)
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if name == "" {
		name = fs.Arg(0)
	}
	if name == "" {
		return fmt.Errorf("usage: lark-switch rm <name> [--purge]")
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	a := s.find(name)
	if a == nil {
		return fmt.Errorf("no such account %q", name)
	}
	dir := expandHome(a.Dir)
	if *purge && isDefaultHome(dir) {
		return fmt.Errorf("refusing to --purge the default home %s", shortDir(dir))
	}
	s.remove(name)
	if err := s.save(); err != nil {
		return err
	}
	fmt.Printf("unregistered %q\n", name)
	if *purge {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removed registry entry but failed to delete %s: %w", shortDir(dir), err)
		}
		fmt.Printf("deleted %s\n", shortDir(dir))
	} else {
		fmt.Printf("(config home %s left intact; pass --purge to delete it)\n", shortDir(dir))
	}
	return nil
}

func cmdCurrent(args []string) error {
	fs := flag.NewFlagSet("current", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "machine-readable JSON output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := loadStore()
	if err != nil {
		return err
	}
	name := currentName(s)
	if name == "" {
		dir := os.Getenv("LARKSUITE_CLI_CONFIG_DIR")
		if dir == "" {
			dir = defaultHome()
		}
		if *jsonOut {
			b, _ := json.Marshal(struct {
				Name *string `json:"name"`
				Dir  string  `json:"dir"`
			}{nil, dir})
			fmt.Println(string(b))
			return nil
		}
		fmt.Printf("(unregistered) %s\n", shortDir(dir))
		return nil
	}
	a := s.find(name)
	if *jsonOut {
		b, _ := json.Marshal(struct {
			Name string `json:"name"`
			Dir  string `json:"dir"`
		}{name, expandHome(a.Dir)})
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%s\t%s\n", name, shortDir(a.Dir))
	return nil
}

func cmdPath(args []string) error {
	s, err := loadStore()
	if err != nil {
		return err
	}
	name := ""
	if len(args) > 0 {
		name = args[0]
	} else {
		name = currentName(s)
	}
	a := s.find(name)
	if a == nil {
		return fmt.Errorf("no such account %q", name)
	}
	fmt.Println(expandHome(a.Dir))
	return nil
}

func cmdShellenv(args []string) error {
	fmt.Print(shimZsh)
	return nil
}

// helpers ---------------------------------------------------------------------

func runInteractive(a *Account, larkArgs ...string) error {
	larkCli, err := larkCliPath()
	if err != nil {
		return err
	}
	c := exec.Command(larkCli, larkArgs...)
	c.Env = envForAccount(a)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c.Run()
}

func larkCliPath() (string, error) {
	p, err := exec.LookPath("lark-cli")
	if err != nil {
		return "", fmt.Errorf("lark-cli not found on PATH (install it first)")
	}
	return p, nil
}

func stripDashDash(a []string) []string {
	if len(a) > 0 && a[0] == "--" {
		return a[1:]
	}
	return a
}

// splitNameFlags peels a leading positional argument (the account name) off the
// front so flags may follow it — Go's flag package otherwise stops parsing at the
// first non-flag token. If args begins with a flag, name is "" and the caller
// falls back to fs.Arg(0).
func splitNameFlags(args []string) (name string, rest []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func shortDir(p string) string {
	p = expandHome(p)
	h := homeDir()
	if h != "" && strings.HasPrefix(p, h+"/") {
		return "~" + p[len(h):]
	}
	return p
}

// isWide reports whether a rune renders as two terminal columns (CJK, etc.).
func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F, // Hangul Jamo
		r >= 0x2E80 && r <= 0x303E, // CJK radicals, Kangxi, symbols
		r >= 0x3041 && r <= 0x33FF, // Hiragana .. CJK compat
		r >= 0x3400 && r <= 0x4DBF, // CJK Ext A
		r >= 0x4E00 && r <= 0x9FFF, // CJK Unified
		r >= 0xA000 && r <= 0xA4CF, // Yi
		r >= 0xAC00 && r <= 0xD7A3, // Hangul Syllables
		r >= 0xF900 && r <= 0xFAFF, // CJK Compat Ideographs
		r >= 0xFE30 && r <= 0xFE4F, // CJK Compat Forms
		r >= 0xFF00 && r <= 0xFF60, // Fullwidth Forms
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD: // CJK Ext B+
		return true
	}
	return false
}

func dispWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

// cell renders s padded (or truncated with an ellipsis) to display width w,
// accounting for double-width CJK characters so columns line up.
func cell(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if dispWidth(s) <= w {
		return s + strings.Repeat(" ", w-dispWidth(s))
	}
	out := make([]rune, 0, len(s))
	cur := 0
	for _, r := range s {
		rw := 1
		if isWide(r) {
			rw = 2
		}
		if cur+rw > w-1 {
			break
		}
		out = append(out, r)
		cur += rw
	}
	pad := w - cur - 1
	if pad < 0 {
		pad = 0
	}
	return string(out) + "…" + strings.Repeat(" ", pad)
}

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return stdoutIsTTY()
}

func stdoutIsTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func red(s string) string  { return "\x1b[31m" + s + "\x1b[0m" }
func bold(s string) string { return "\x1b[1m" + s + "\x1b[0m" }

const shimZsh = `# lark-switch shell integration — add to ~/.zshrc (or ~/.bashrc):
#   eval "$(lark-switch shellenv)"
# then:  lark-switch use B   |   lk use B   |   lk run A -- im +chat-list
lark-switch() {
  case "$1" in
    use)
      local _e
      _e="$(LARK_SWITCH_EVAL=1 command lark-switch use "${@:2}")" || return $?
      eval "$_e"
      ;;
    *)
      command lark-switch "$@"
      ;;
  esac
}
lk() { lark-switch "$@"; }
`

func usage() {
	fmt.Print(`lark-switch — gvm-style multi-account switcher for lark-cli (Lark/Feishu CLI)

USAGE:
  lark-switch <command> [args]

COMMANDS:
  add <name> [--dir <path>] [--init] [--domain ...]
                         register an account; --init runs config init + auth login
  login <name> [--domain ...|--scope ...] [--init]
                         (re)authorize an account
  ls [--json]            list accounts with user, token status, refresh window
  use <name>             switch the current shell to <name>   (needs the shim; see shellenv)
  run <name> -- <args>   run one lark-cli command as <name> (no global state change)
  each -- <args>         run a lark-cli command across all accounts
  refresh [<name>|--all] keep tokens alive (cron this; tokens lapse after ~7 idle days)
  current [--json]       show the account active in this shell (alias: which)
  path [<name>]          print an account's config home dir
  rm <name> [--purge]    unregister (and optionally delete its home)
  shellenv               print the shell shim (eval "$(lark-switch shellenv)")
  version

NOTES:
  • Each account is pinned to its own LARKSUITE_CLI_CONFIG_DIR home; the default
    home (~/.lark-cli) account never has that var set (its tokens live elsewhere).
  • AI agents / scripts: use 'run' (stateless) — never 'use', which needs the shell
    shim and exits 1 elsewhere. Discover accounts with 'ls --json'. See AGENTS.md.
`)
}
