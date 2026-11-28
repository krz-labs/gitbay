package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/krz-labs/gitbay/internal/cliconfig"
	"github.com/krz-labs/gitbay/internal/protocol"
	"github.com/krz-labs/gitbay/internal/toolpath"
)

// parseNumberArg parses a positive integer, optionally prefixed with
// prefix ("#" for issues, "!" for MRs).
func parseNumberArg(s, prefix string) (string, bool) {
	if prefix != "" {
		s = strings.TrimPrefix(s, prefix)
	}
	if s == "" {
		return "", false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return "", false
	}
	return strconv.FormatInt(n, 10), true
}

// withDefault attaches default list/show routing to a noun group so
// `gitbay issue` lists, `gitbay issue 42` (or `#42`) shows, and
// `gitbay issue --state all` lists with flags. Explicit `--help` / `-h`
// still prints the server's noun help.
func withDefault(cmd *cobra.Command, listOpts, showOpts passOpts, numPrefix string) *cobra.Command {
	cmd.DisableFlagParsing = true
	cmd.Args = cobra.ArbitraryArgs
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			return c.Help()
		}
		cliBase := cliPathOf(c)
		if len(args) >= 1 {
			if num, ok := parseNumberArg(args[0], numPrefix); ok {
				cp := append([]string{num}, args[1:]...)
				os.Exit(runPass(showOpts, cliBase+" show", cp))
			}
		}
		if len(args) >= 2 && !strings.HasPrefix(args[0], "-") && strings.Contains(args[0], "/") {
			if num, ok := parseNumberArg(args[1], numPrefix); ok {
				cp := slices.Clone(args)
				cp[1] = num
				os.Exit(runPass(showOpts, cliBase+" show", cp))
			}
		}
		os.Exit(runPass(listOpts, cliBase+" list", args))
		return nil
	}
	return cmd
}

// withRepoDefault attaches default routing to `gitbay repo`: inside a
// clone (or with an explicit owner/name), `gitbay repo` shows the repo;
// otherwise it lists repositories.
func withRepoDefault(cmd *cobra.Command) *cobra.Command {
	cmd.DisableFlagParsing = true
	cmd.Args = cobra.ArbitraryArgs
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if slices.Contains(args, "--help") || slices.Contains(args, "-h") {
			return c.Help()
		}
		showOpts := passOpts{server: []string{"repo", "show"}, needsRepo: true}
		listOpts := passOpts{server: []string{"repo", "list"}}
		if len(args) > 0 && !strings.HasPrefix(args[0], "-") && strings.Contains(args[0], "/") {
			os.Exit(runPass(showOpts, "repo show", args))
		}
		if len(args) == 0 {
			if t, err := resolveTarget(); err == nil && t.repo != "" {
				os.Exit(runPass(showOpts, "repo show", nil))
			}
		}
		os.Exit(runPass(listOpts, "repo list", args))
		return nil
	}
	return cmd
}

// hasPositionalAfterRepo reports whether args (whose args[0] is already
// the repository path) contains a positional argument after args[0].
func hasPositionalAfterRepo(args []string) bool {
	if len(args) <= 1 {
		return false
	}
	for i := 1; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return i+1 < len(args)
		}
		if strings.HasPrefix(a, "--") {
			if !strings.Contains(a, "=") && flagTakesValue(a) {
				i++
			}
			continue
		}
		if len(a) == 2 && a[0] == '-' {
			if shortFlagTakesValue(a) {
				i++
			}
			continue
		}
		return true
	}
	return false
}

// flagTakesValue reports whether a long flag in mr/build commands consumes
// the following argument. Boolean flags return false.
func flagTakesValue(f string) bool {
	switch f {
	case "--json", "--draft", "--when-ready", "--cancel", "--follow",
		"--approve", "--request-changes", "--comment", "--discard",
		"--pending", "--old", "--all", "--no-color":
		return false
	}
	return true
}

func shortFlagTakesValue(f string) bool {
	switch f {
	case "-d":
		return false
	case "-t", "-m", "-b", "-s", "-l", "-a", "-f", "-n":
		return true
	}
	return false
}

// stripLeadingNumberPrefix normalizes "#42" or "!12" at args[1] (right
// after the repo path) to plain digits.
func stripLeadingNumberPrefix(args []string, prefix string) []string {
	if prefix == "" || len(args) < 2 {
		return args
	}
	if num, ok := parseNumberArg(args[1], prefix); ok && num != args[1] {
		cp := slices.Clone(args)
		cp[1] = num
		return cp
	}
	return args
}

// inferBranchMR resolves the open MR number for the currently checked-out
// branch in t.repo.
func inferBranchMR(t target) (string, error) {
	br := currentBranch()
	if br == "" {
		return "", fmt.Errorf("not on a branch; pass an MR number")
	}
	out, code := sshCapture(t, []string{"mr", "list", t.repo, "--state", "open", "--json"})
	if code != 0 {
		return "", fmt.Errorf("could not list open MRs for %s", t.repo)
	}
	var env struct {
		Data []struct {
			Number int64  `json:"number"`
			Source string `json:"source"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		return "", fmt.Errorf("reading open MRs: %w", err)
	}
	for _, mr := range env.Data {
		if mr.Source == br {
			return strconv.FormatInt(mr.Number, 10), nil
		}
	}
	return "", fmt.Errorf("no open MR for branch %q; pass <n> or run: gitbay mr create", br)
}

// inferLatestBuild resolves the most recent build number for the current
// branch (or repository) in t.repo.
func inferLatestBuild(t target) (string, error) {
	argv := []string{"build", "list", t.repo, "--limit", "1", "--json"}
	if br := currentBranch(); br != "" {
		argv = []string{"build", "list", t.repo, "--ref", br, "--limit", "1", "--json"}
	}
	out, code := sshCapture(t, argv)
	if code != 0 {
		return "", fmt.Errorf("could not list builds for %s", t.repo)
	}
	var env struct {
		Data struct {
			Items []struct {
				Number int64 `json:"number"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil || len(env.Data.Items) == 0 {
		// Fall back to latest build on any ref when branch has no builds yet.
		if br := currentBranch(); br != "" {
			out2, code2 := sshCapture(t, []string{"build", "list", t.repo, "--limit", "1", "--json"})
			if code2 == 0 && json.Unmarshal([]byte(out2), &env) == nil && len(env.Data.Items) > 0 {
				return strconv.FormatInt(env.Data.Items[0].Number, 10), nil
			}
		}
		return "", fmt.Errorf("no builds found for %s; pass a build number", t.repo)
	}
	return strconv.FormatInt(env.Data.Items[0].Number, 10), nil
}

// headCommitTitleBody returns the checked-out commit's subject and body,
// or empty strings outside a git repo.
func headCommitTitleBody() (string, string) {
	subj, err := exec.Command(toolpath.Look("git"), "log", "-1", "--format=%s").Output()
	if err != nil {
		return "", ""
	}
	body, _ := exec.Command(toolpath.Look("git"), "log", "-1", "--format=%b").Output()
	return strings.TrimSpace(string(subj)), strings.TrimSpace(string(body))
}

// splitTitleAndBody extracts the first non-comment, non-blank line as
// title and the remaining non-comment lines as body.
func splitTitleAndBody(raw string) (title, body string) {
	var kept []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		kept = append(kept, line)
	}
	start := 0
	for start < len(kept) && strings.TrimSpace(kept[start]) == "" {
		start++
	}
	if start >= len(kept) {
		return "", ""
	}
	title = strings.TrimSpace(kept[start])
	rest := strings.TrimSpace(strings.Join(kept[start+1:], "\n"))
	return title, rest
}

// maybeTitleAndBodyEditor opens $EDITOR when --title/-t is missing on
// issue create or mr create, letting the user write title on line 1 and
// body below. If no editor is configured or stdin is not a terminal,
// falls back to prefillTitle when available.
func maybeTitleAndBodyEditor(args []string, kind, prefillTitle, prefillBody string) ([]string, *strings.Reader, bool, error) {
	if hasFlag(args, "--title") || hasFlag(args, "-t") {
		return args, nil, true, nil
	}
	editor := os.Getenv("EDITOR")
	if hasBodyFlag(args) || !term.IsTerminal(int(os.Stdin.Fd())) || editor == "" {
		if prefillTitle != "" {
			return append(args, "--title", prefillTitle), nil, true, nil
		}
		return args, nil, true, nil
	}
	f, err := os.CreateTemp("", "gitbay-"+kind+"-*.md")
	if err != nil {
		return nil, nil, false, err
	}
	defer os.Remove(f.Name())
	if prefillTitle != "" {
		fmt.Fprintf(f, "%s\n", prefillTitle)
	}
	if prefillBody != "" {
		fmt.Fprintf(f, "\n%s\n", strings.TrimRight(prefillBody, "\n"))
	}
	fmt.Fprintf(f, "\n# First line is the %s title; leave a blank line and write the body below.\n# Lines starting with '#' are ignored. Save an empty file to abort.\n", kind)
	f.Close()

	ed := exec.Command(toolpath.Look("sh"), "-c", editor+" "+shellQuote(f.Name()))
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return nil, nil, false, fmt.Errorf("editor: %w", err)
	}
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		return nil, nil, false, err
	}
	title, body := splitTitleAndBody(string(raw))
	if title == "" {
		fmt.Fprintln(os.Stderr, "gitbay: aborted (empty title)")
		return nil, nil, false, nil
	}
	args = append(args, "--title", title)
	if body == "" {
		return args, nil, true, nil
	}
	return append(args, "--file", "-"), strings.NewReader(body + "\n"), true, nil
}

// instanceBaseURL derives the web base URL for an instance.
func instanceBaseURL(inst cliconfig.Instance) string {
	scheme := "https"
	if inst.Host == "localhost" || inst.Host == "127.0.0.1" || strings.HasSuffix(inst.Host, ".local") {
		scheme = "http"
	}
	return scheme + "://" + inst.Host
}

// resolveOpenURL maps `gitbay open` arguments to a web URL on the target
// instance. inferMR and inferBuild are injected for testability.
func resolveOpenURL(t target, args []string, branch string, stat func(string) (os.FileInfo, error), inferMR, inferBuild func() (string, error)) (string, error) {
	base := instanceBaseURL(t.inst)
	if len(args) == 0 {
		if t.repo == "" {
			return base + "/", nil
		}
		return base + "/" + t.repo, nil
	}
	arg := args[0]
	repo := t.repo

	// Explicit owner/name as first argument.
	if !strings.HasPrefix(arg, "#") && !strings.HasPrefix(arg, "!") && strings.Count(arg, "/") == 1 {
		if _, err := stat(arg); err != nil {
			repo = arg
			args = args[1:]
			if len(args) == 0 {
				return base + "/" + repo, nil
			}
			arg = args[0]
		}
	}

	if repo == "" {
		return "", fmt.Errorf("no repository given and none inferable: pass <owner/name> or run inside a clone")
	}

	if num, ok := parseNumberArg(arg, "#"); ok && strings.HasPrefix(arg, "#") {
		return fmt.Sprintf("%s/%s/issues/%s", base, repo, num), nil
	}
	if num, ok := parseNumberArg(arg, "!"); ok && strings.HasPrefix(arg, "!") {
		return fmt.Sprintf("%s/%s/mrs/%s", base, repo, num), nil
	}
	switch arg {
	case "issues", "mrs", "builds", "releases", "wiki", "settings":
		return fmt.Sprintf("%s/%s/%s", base, repo, arg), nil
	case "mr":
		if len(args) > 1 {
			if num, ok := parseNumberArg(args[1], "!"); ok {
				return fmt.Sprintf("%s/%s/mrs/%s", base, repo, num), nil
			}
		}
		if inferMR != nil {
			if num, err := inferMR(); err == nil {
				return fmt.Sprintf("%s/%s/mrs/%s", base, repo, num), nil
			}
		}
		return fmt.Sprintf("%s/%s/mrs", base, repo), nil
	case "build":
		if len(args) > 1 {
			if num, ok := parseNumberArg(args[1], ""); ok {
				return fmt.Sprintf("%s/%s/builds/%s", base, repo, num), nil
			}
		}
		if inferBuild != nil {
			if num, err := inferBuild(); err == nil {
				return fmt.Sprintf("%s/%s/builds/%s", base, repo, num), nil
			}
		}
		return fmt.Sprintf("%s/%s/builds", base, repo), nil
	}

	// File or directory path, optionally with :line.
	pathPart, linePart, hasLine := strings.Cut(arg, ":")
	cleanPath := filepath.ToSlash(filepath.Clean(pathPart))
	fi, err := stat(pathPart)
	if err == nil {
		ref := branch
		if ref == "" {
			ref = "HEAD"
		}
		kind := "blob"
		if fi.IsDir() {
			kind = "tree"
		}
		u := fmt.Sprintf("%s/%s/%s/%s/%s", base, repo, kind, url.PathEscape(ref), cleanPath)
		if hasLine && linePart != "" {
			if n, err := strconv.Atoi(linePart); err == nil && n > 0 {
				u += fmt.Sprintf("#L%d", n)
			}
		}
		return u, nil
	}

	return "", fmt.Errorf("cannot resolve %q (use #N, !N, mr, build [N], issues|mrs|builds|releases|wiki|settings, owner/name, or a file path)", arg)
}

func openCmd() *cobra.Command {
	c := local("open", "open the repository, issue, MR, build, or file in your browser", cmdOpen)
	c.Aliases = []string{"browse"}
	return c
}

func cmdOpen(args []string) int {
	printOnly := false
	var rest []string
	for _, a := range args {
		if a == "--print" || a == "-p" {
			printOnly = true
			continue
		}
		rest = append(rest, a)
	}
	t, err := resolveTarget()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitbay:", err)
		return protocol.ExitFailure
	}
	u, err := resolveOpenURL(t, rest, currentBranch(), os.Stat,
		func() (string, error) { return inferBranchMR(t) },
		func() (string, error) { return inferLatestBuild(t) },
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gitbay:", err)
		return protocol.ExitUsage
	}
	if printOnly {
		fmt.Println(u)
		return 0
	}
	if err := openBrowser(u); err != nil {
		fmt.Fprintln(os.Stderr, "gitbay: opening browser:", err)
		fmt.Println(u)
		return protocol.ExitFailure
	}
	return 0
}

func openBrowser(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", u).Start()
	default:
		return exec.Command("xdg-open", u).Start()
	}
}
