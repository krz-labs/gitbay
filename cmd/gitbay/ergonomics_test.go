package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"gitbay.org/gitbay/internal/cliconfig"
)

func TestParseNumberArg(t *testing.T) {
	cases := []struct {
		in, prefix, want string
		ok               bool
	}{
		{"42", "#", "42", true},
		{"#42", "#", "42", true},
		{"!12", "!", "12", true},
		{"#12", "!", "", false},
		{"0", "#", "", false},
		{"-1", "#", "", false},
		{"abc", "", "", false},
	}
	for _, tc := range cases {
		got, ok := parseNumberArg(tc.in, tc.prefix)
		if got != tc.want || ok != tc.ok {
			t.Errorf("parseNumberArg(%q, %q) = (%q, %v), want (%q, %v)", tc.in, tc.prefix, got, ok, tc.want, tc.ok)
		}
	}
}

func TestHasPositionalAfterRepo(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"krz/gitbay"}, false},
		{[]string{"krz/gitbay", "--strategy", "ff"}, false},
		{[]string{"krz/gitbay", "--when-ready", "--json"}, false},
		{[]string{"krz/gitbay", "-m", "looks good"}, false},
		{[]string{"krz/gitbay", "42"}, true},
		{[]string{"krz/gitbay", "--strategy", "ff", "42"}, true},
		{[]string{"krz/gitbay", "--strategy=ff", "42"}, true},
		{[]string{"krz/gitbay", "--follow"}, false},
	}
	for _, tc := range cases {
		if got := hasPositionalAfterRepo(tc.args); got != tc.want {
			t.Errorf("hasPositionalAfterRepo(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestStripLeadingNumberPrefix(t *testing.T) {
	got := stripLeadingNumberPrefix([]string{"krz/gitbay", "#42", "--json"}, "#")
	if !slices.Equal(got, []string{"krz/gitbay", "42", "--json"}) {
		t.Errorf("got %v", got)
	}
	got = stripLeadingNumberPrefix([]string{"krz/gitbay", "!7"}, "!")
	if !slices.Equal(got, []string{"krz/gitbay", "7"}) {
		t.Errorf("got %v", got)
	}
}

func TestSplitTitleAndBody(t *testing.T) {
	raw := "# comment\n\nFix the thing\n\nMore details\n# ignored\nSecond line\n"
	title, body := splitTitleAndBody(raw)
	if title != "Fix the thing" || body != "More details\nSecond line" {
		t.Errorf("got title=%q body=%q", title, body)
	}
	title, body = splitTitleAndBody("# only comments\n\n")
	if title != "" || body != "" {
		t.Errorf("empty: got title=%q body=%q", title, body)
	}
}

func TestShortFlagHelpers(t *testing.T) {
	if !usesStdin([]string{"-f", "-"}) || !usesStdin([]string{"--file=-"}) {
		t.Errorf("usesStdin should recognize -f - and --file=-")
	}
	if !hasBodyFlag([]string{"-b", "hi"}) || !hasBodyFlag([]string{"-m", "hi"}) || !hasBodyFlag([]string{"--body=hi"}) {
		t.Errorf("hasBodyFlag should recognize -b, -m, and --body=")
	}
}

func TestResolveOpenURL(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirPath := filepath.Join(tmp, "sub")
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatal(err)
	}

	tgt := target{inst: cliconfig.Instance{Host: "gitbay.org"}, repo: "krz/gitbay"}
	inferMR := func() (string, error) { return "42", nil }
	inferBuild := func() (string, error) { return "99", nil }

	cases := []struct {
		t    target
		args []string
		want string
	}{
		{target{inst: cliconfig.Instance{Host: "gitbay.org"}}, nil, "https://gitbay.org/"},
		{tgt, nil, "https://gitbay.org/krz/gitbay"},
		{tgt, []string{"#15"}, "https://gitbay.org/krz/gitbay/issues/15"},
		{tgt, []string{"!8"}, "https://gitbay.org/krz/gitbay/mrs/8"},
		{tgt, []string{"issues"}, "https://gitbay.org/krz/gitbay/issues"},
		{tgt, []string{"mr"}, "https://gitbay.org/krz/gitbay/mrs/42"},
		{tgt, []string{"mr", "!7"}, "https://gitbay.org/krz/gitbay/mrs/7"},
		{tgt, []string{"build"}, "https://gitbay.org/krz/gitbay/builds/99"},
		{tgt, []string{"build", "12"}, "https://gitbay.org/krz/gitbay/builds/12"},
		{tgt, []string{"cmc/other", "#3"}, "https://gitbay.org/cmc/other/issues/3"},
		{tgt, []string{filePath + ":10"}, "https://gitbay.org/krz/gitbay/blob/feat/" + filepath.ToSlash(filePath) + "#L10"},
		{tgt, []string{dirPath}, "https://gitbay.org/krz/gitbay/tree/feat/" + filepath.ToSlash(dirPath)},
		{target{inst: cliconfig.Instance{Host: "localhost"}, repo: "a/b"}, nil, "http://localhost/a/b"},
	}
	for _, tc := range cases {
		got, err := resolveOpenURL(tc.t, tc.args, "feat", os.Stat, inferMR, inferBuild)
		if err != nil {
			t.Errorf("resolveOpenURL(%v): %v", tc.args, err)
			continue
		}
		if got != tc.want {
			t.Errorf("resolveOpenURL(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}
