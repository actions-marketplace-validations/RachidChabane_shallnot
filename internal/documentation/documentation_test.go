// Package documentation_test checks that the user-facing documentation names
// what the code does.
package documentation_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/RachidChabane/shallnot/internal/adapters/agenthook"
	"github.com/RachidChabane/shallnot/internal/adapters/scaffold"
	"github.com/RachidChabane/shallnot/internal/cli"
)

const repositoryRoot = "../.."

// harnessNames is how the documentation names each harness `shallnot init`
// can install an end-of-turn hook for, by its `--hooks` name.
var harnessNames = map[string]string{
	"claude":   "Claude Code",
	"codex":    "Codex",
	"copilot":  "GitHub Copilot",
	"cursor":   "Cursor",
	"factory":  "Factory Droid",
	"gemini":   "Gemini CLI",
	"goose":    "Goose",
	"opencode": "OpenCode",
	"qwen":     "Qwen Code",
}

const (
	// hookSentence opens the passage that says which harnesses get a hook.
	hookSentence       = "end-of-turn hook for Claude Code"
	hooksHeading       = "### Hooks `shallnot init` can install"
	coverageHeading    = "## Hook and instruction coverage"
	flagsHeading       = "### Flags"
	hookedCoverage     = "hook installed by `init`"
	parenthesisOpening = " ("
)

var (
	hooksFlagList  = regexp.MustCompile(`harness names \(([^)]*)\)`)
	codeSpan       = regexp.MustCompile("`([^`]+)`")
	hookInvocation = regexp.MustCompile(`shallnot hook ([a-z][a-z-]*)`)
)

func read(t *testing.T, name string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRoot, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

// flatten joins the lines of a text, so that a name wrapped across two lines reads as one.
func flatten(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func mentions(text, name string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(text)
}

// hookedHarnesses returns the documented name of each harness init can hook, by its `--hooks` name.
func hookedHarnesses(t *testing.T) map[string]string {
	t.Helper()
	documented := make([]string, 0, len(harnessNames))
	for harness := range harnessNames {
		documented = append(documented, harness)
	}
	sort.Strings(documented)
	if installable := scaffold.HookHarnesses(); !reflect.DeepEqual(documented, installable) {
		t.Fatalf("init installs hooks for %v; this test names %v", installable, documented)
	}
	return harnessNames
}

// section returns the text under a Markdown heading, up to the next heading of the same or a higher level.
func section(t *testing.T, markdown, heading string) string {
	t.Helper()
	level := headingLevel(heading)
	lines := strings.Split(markdown, "\n")
	for start, line := range lines {
		if line != heading {
			continue
		}
		end := start + 1
		for end < len(lines) && !closesSection(lines[end], level) {
			end++
		}
		return strings.Join(lines[start+1:end], "\n")
	}
	t.Fatalf("no heading %q", heading)
	return ""
}

// headingLevel is the number of `#` that open a Markdown heading line, or 0 for any other line.
func headingLevel(line string) int {
	depth := len(line) - len(strings.TrimLeft(line, "#"))
	if depth == 0 || !strings.HasPrefix(line[depth:], " ") {
		return 0
	}
	return depth
}

func closesSection(line string, level int) bool {
	depth := headingLevel(line)
	return depth > 0 && depth <= level
}

// tableRows returns the cells of each body row of the first table in a text.
func tableRows(t *testing.T, text string) [][]string {
	t.Helper()
	var rows [][]string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "|") {
			if len(rows) > 0 {
				break
			}
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for index := range cells {
			cells[index] = strings.TrimSpace(cells[index])
		}
		rows = append(rows, cells)
	}
	if len(rows) < 2 {
		t.Fatal("no table")
	}
	return rows[2:]
}

// harnessOfRow returns the `--hooks` name of the harness a table row names.
func harnessOfRow(t *testing.T, harnesses map[string]string, row []string) string {
	t.Helper()
	var matched []string
	for harness, name := range harnesses {
		if mentions(row[0], name) {
			matched = append(matched, harness)
		}
	}
	if len(matched) != 1 {
		t.Fatalf("row %q names %d harnesses init can hook, not 1", row[0], len(matched))
	}
	return matched[0]
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// unhookedHarnesses returns the names of the harnesses docs/agents.md covers without an init hook.
func unhookedHarnesses(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, row := range tableRows(t, section(t, read(t, "docs/agents.md"), coverageHeading)) {
		if !strings.HasPrefix(row[1], hookedCoverage) {
			name, _, _ := strings.Cut(row[0], parenthesisOpening)
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatal("the coverage table names no harness without a hook")
	}
	return names
}

// assertHookPassage checks that the passage saying which harnesses get a hook names exactly those init can hook.
func assertHookPassage(t *testing.T, document string) {
	t.Helper()
	var passages []string
	for _, paragraph := range strings.Split(read(t, document), "\n\n") {
		if flat := flatten(paragraph); strings.Contains(flat, hookSentence) {
			passages = append(passages, flat)
		}
	}
	if len(passages) != 1 {
		t.Fatalf("%s: %d paragraphs contain %q, not 1", document, len(passages), hookSentence)
	}
	for harness, name := range hookedHarnesses(t) {
		if !mentions(passages[0], name) {
			t.Errorf("%s does not name %s (%s) among the harnesses that get a hook", document, name, harness)
		}
	}
	for _, name := range unhookedHarnesses(t) {
		if mentions(passages[0], name) {
			t.Errorf("%s names %s among the harnesses that get a hook", document, name)
		}
	}
}

func TestHarnesses(t *testing.T) {
	t.Run("the README names the harnesses init installs a hook for [verifies SN-84~1]", func(t *testing.T) {
		assertHookPassage(t, "README.md")
	})

	t.Run("llms.txt names the harnesses init installs a hook for [verifies SN-84~1]", func(t *testing.T) {
		assertHookPassage(t, "llms.txt")
	})

	t.Run("the hooks table of docs/agents.md has one row per harness init installs a hook for [verifies SN-84~1]", func(t *testing.T) {
		harnesses := hookedHarnesses(t)
		rows := tableRows(t, section(t, read(t, "docs/agents.md"), hooksHeading))
		listed := map[string]bool{}
		for _, row := range rows {
			listed[harnessOfRow(t, harnesses, row)] = true
		}
		if got := sortedKeys(listed); len(rows) != len(harnesses) || !reflect.DeepEqual(got, scaffold.HookHarnesses()) {
			t.Fatalf("%d rows for %v; init installs hooks for %v", len(rows), got, scaffold.HookHarnesses())
		}
	})

	t.Run("the coverage table of docs/agents.md says init hooks exactly the harnesses it installs a hook for [verifies SN-84~1]", func(t *testing.T) {
		harnesses := hookedHarnesses(t)
		hooked := map[string]bool{}
		for _, row := range tableRows(t, section(t, read(t, "docs/agents.md"), coverageHeading)) {
			if strings.HasPrefix(row[1], hookedCoverage) {
				hooked[harnessOfRow(t, harnesses, row)] = true
			}
		}
		if got := sortedKeys(hooked); !reflect.DeepEqual(got, scaffold.HookHarnesses()) {
			t.Fatalf("hooked in the table: %v; init installs hooks for %v", got, scaffold.HookHarnesses())
		}
	})

	t.Run("docs/agents.md lists the names the --hooks flag accepts [verifies SN-84~1]", func(t *testing.T) {
		list := hooksFlagList.FindStringSubmatch(flatten(section(t, read(t, "docs/agents.md"), flagsHeading)))
		if list == nil {
			t.Fatal("no list of harness names under the flags")
		}
		names := map[string]bool{}
		for _, span := range codeSpan.FindAllStringSubmatch(list[1], -1) {
			names[span[1]] = true
		}
		if got := sortedKeys(names); !reflect.DeepEqual(got, scaffold.HookHarnesses()) {
			t.Fatalf("documented %v; init accepts %v", got, scaffold.HookHarnesses())
		}
	})
}

func TestHooks(t *testing.T) {
	t.Run("docs/agents.md documents each hook shallnot hook answers and no other [verifies SN-85~1]", func(t *testing.T) {
		documented := map[string]bool{}
		for _, invocation := range hookInvocation.FindAllStringSubmatch(flatten(read(t, "docs/agents.md")), -1) {
			documented[invocation[1]] = true
		}
		if got := sortedKeys(documented); !reflect.DeepEqual(got, agenthook.Names()) {
			t.Fatalf("documented %v; shallnot hook answers %v", got, agenthook.Names())
		}
	})

	t.Run("the usage text lists each hook shallnot hook answers and no other [verifies SN-85~1]", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cli.Main([]string{"help"}, strings.NewReader(""), &stdout, &stderr)
		listed := map[string]bool{}
		for _, line := range strings.Split(stdout.String(), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "shallnot hook ") {
				_, names, _ := strings.Cut(line, ": ")
				for _, name := range strings.Split(names, ", ") {
					listed[name] = true
				}
			}
		}
		if got := sortedKeys(listed); !reflect.DeepEqual(got, agenthook.Names()) {
			t.Fatalf("the usage lists %v; shallnot hook answers %v", got, agenthook.Names())
		}
	})
}
