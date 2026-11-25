package control

import (
	"strings"
	"testing"
)

func TestParseFlags(t *testing.T) {
	spec := flagSpec{Values: []string{"--title", "--file"}, Multi: []string{"--add"}, Bools: []string{"--private"}, MaxPos: 1, Usage: "cmd <owner/name> [--title <t>]"}
	f, err := parseFlags([]string{"a/b", "--title", "-x", "--add", "one", "--private", "--add", "two", "--file", "-"}, spec)
	if err != nil {
		t.Fatal(err)
	}
	if f.pos(0) != "a/b" || f.Value("--title") != "-x" || f.Value("--file") != "-" || !f.Has("--private") ||
		strings.Join(f.List("--add"), ",") != "one,two" || f.Has("--nope") || f.pos(1) != "" {
		t.Fatalf("parsed wrong: %+v", f)
	}
	for _, bad := range [][]string{
		{"a/b", "--bogus"},        // unknown flag
		{"a/b", "--title"},        // missing value
		{"a/b", "c/d"},            // too many positionals
		{"--private", "a/b", "x"}, // too many, flags first
	} {
		if _, err := parseFlags(bad, spec); err == nil || !strings.Contains(err.Error(), "usage: cmd") {
			t.Errorf("%v: err=%v", bad, err)
		}
	}
	// "--" ends flags; MaxPos -1 takes any number.
	f, err = parseFlags([]string{"--", "--title", "x"}, flagSpec{Values: []string{"--title"}, MaxPos: -1})
	if err != nil || strings.Join(f.Pos, " ") != "--title x" || f.Has("--title") {
		t.Fatalf("-- handling: %v %+v", err, f)
	}

	// Short flags and --flag=value forms.
	shortSpec := flagSpec{
		Values: []string{"--title", "--state", "--file", "--limit"},
		Multi:  []string{"--label"},
		Bools:  []string{"--draft"},
		MaxPos: 1,
	}
	f, err = parseFlags([]string{"a/b", "-t", "hello", "-s", "all", "-l", "bug", "-l", "ux", "-d", "-f", "-", "-n", "10"}, shortSpec)
	if err != nil {
		t.Fatalf("short flags: %v", err)
	}
	if f.Value("--title") != "hello" || f.Value("--state") != "all" || f.Value("--file") != "-" ||
		f.Value("--limit") != "10" || !f.Has("--draft") || strings.Join(f.List("--label"), ",") != "bug,ux" {
		t.Fatalf("short flags parsed wrong: %+v", f)
	}
	f, err = parseFlags([]string{"a/b", "--title=hello=world", "--label=bug", "--label=ux"}, shortSpec)
	if err != nil {
		t.Fatalf("--flag=value: %v", err)
	}
	if f.Value("--title") != "hello=world" || strings.Join(f.List("--label"), ",") != "bug,ux" {
		t.Fatalf("--flag=value parsed wrong: %+v", f)
	}
	if _, err := parseFlags([]string{"a/b", "--draft=true"}, shortSpec); err == nil {
		t.Errorf("expected error for --bool=value")
	}
}

func TestNearestFlag(t *testing.T) {
	known := map[string]byte{"--state": 'v', "--label": 'm', "--limit": 'v', "--json": 'b'}
	cases := map[string]string{
		"--stat":    "--state",
		"--lable":   "--label",
		"--l":       "",
		"--jsno":    "--json",
		"--nothing": "",
	}
	for in, want := range cases {
		if got := nearestFlag(in, known); got != want {
			t.Errorf("nearestFlag(%q) = %q, want %q", in, got, want)
		}
	}
}
