package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// validateCreds is a seam for tests; production hits the provider API.
var validateCreds = func(ctx context.Context, v provider.CredentialValidator,
	cfg config.Config) error {
	return v.ValidateCreds(ctx, cfg)
}

// lookupSSH is a seam for tests; production checks PATH.
var lookupSSH = func() error {
	_, err := exec.LookPath("ssh")
	return err
}

// checkResult is one doctor row: aligned cells, an optional faint hint
// line under it, and whether the check counts as a problem.
type checkResult struct {
	row     []string
	hint    string
	problem bool
}

func okRow(name, detail string) []string {
	return []string{ui.OK.Render(ui.GlyphOK) + " " + name, ui.Faint.Render(detail)}
}

func warnRow(name, detail string) []string {
	return []string{ui.Warn.Render(ui.GlyphWarn) + " " + name, ui.Faint.Render(detail)}
}

func failRow(name, detail string) []string {
	return []string{ui.Fail.Render(ui.GlyphFail) + " " + name, ui.Faint.Render(detail)}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "check the tools and credentials interview needs",
		Long: `Check the local environment: terraform or opentofu on PATH, the ssh
client, kind and kubectl, XDG state directories, and stored credentials.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd)
		},
	}
}

// runDoctor renders the TOOLS/CREDENTIALS sections, verdict and NEXT block,
// and reports a silenced error when any check is a problem.
func runDoctor(cmd *cobra.Command) error {
	p := func(s string) { fmt.Fprintln(cmd.OutOrStdout(), s) }

	tools := []checkResult{checkTerraform(), checkSSHClient()}
	tools = append(tools, checkKindTools()...)
	if dirs := checkStateDirs(); dirs != nil {
		tools = append(tools, *dirs)
	}
	renderSection(p, "tools", tools)

	p("")
	creds, anyConfigured := credChecks(cmd.Context())
	renderSection(p, "credentials", creds)

	problems := countProblems(tools, creds)
	p("")
	p(verdictRow(problems))
	if next := doctorNext(problems, countProblems(creds), anyConfigured); next != "" {
		p("")
		p(ui.Next(next))
	}

	if problems > 0 {
		return silenceError(fmt.Errorf("doctor found problems"))
	}
	return nil
}

// renderSection prints one titled block of aligned check rows; a check's
// hint renders as a faint line under its row.
func renderSection(p func(string), title string, checks []checkResult) {
	rows := make([][]string, len(checks))
	for i, c := range checks {
		rows[i] = c.row
	}
	lines := ui.Columns(rows)

	body := make([]string, 0, len(checks)*2)
	for i, c := range checks {
		body = append(body, lines[i])
		if c.hint != "" {
			body = append(body, "  "+ui.Faint.Render(c.hint))
		}
	}
	p(ui.Section(ui.SectionTitle(title), body...))
}

// countProblems totals failing checks across sections.
func countProblems(lists ...[]checkResult) int {
	n := 0
	for _, l := range lists {
		for _, c := range l {
			if c.problem {
				n++
			}
		}
	}
	return n
}

// verdictRow renders the closing state row.
func verdictRow(problems int) string {
	if problems == 0 {
		return ui.OK.Render(ui.GlyphOK) + " ready"
	}
	noun := "problems"
	if problems == 1 {
		noun = "problem"
	}
	return ui.Fail.Render(ui.GlyphFail) + fmt.Sprintf(" %d %s", problems, noun)
}

// doctorNext picks the NEXT command: launch when ready, init when the
// credentials need attention (or nothing is configured yet); tool-only
// failures keep their inline install hint instead.
func doctorNext(problems, credProblems int, anyConfigured bool) string {
	switch {
	case problems == 0 && anyConfigured:
		return "interview launch"
	case credProblems > 0 || !anyConfigured:
		return "interview init"
	default:
		return ""
	}
}

// checkTerraform reports the tf binary row.
func checkTerraform() checkResult {
	bin, err := terraform.Find()
	if err != nil {
		return checkResult{row: failRow("terraform", "not found"),
			hint:    "install terraform or opentofu, then rerun interview doctor",
			problem: true}
	}
	return checkResult{row: okRow(bin.Name, bin.Version+" ("+bin.Path+")")}
}

// checkSSHClient reports the ssh row — a note, never a failure.
func checkSSHClient() checkResult {
	if err := lookupSSH(); err != nil {
		return checkResult{row: warnRow("ssh client",
			"not found — interview ssh unavailable; launch still works")}
	}
	return checkResult{row: okRow("ssh client", "found")}
}

// kindToolVersion is a seam for tests; production runs the binary.
var kindToolVersion = func(bin string) (string, error) {
	args := []string{"version"}
	if bin == "kubectl" {
		args = []string{"version", "--client"}
	}
	out, err := exec.Command(bin, args...).CombinedOutput()
	return string(out), err
}

// kindToolFloors are the minimum families matching the pinned VM toolchain.
var kindToolFloors = map[string][2]int{
	"kind":    {0, 32},
	"kubectl": {1, 35},
}

var toolVersionPattern = regexp.MustCompile(`v(\d+)\.(\d+)\.\d+`)

// checkKindTools reports the kind/kubectl rows — warns, never failures:
// only local kubernetes bundles need them, and launch gates on presence.
func checkKindTools() []checkResult {
	checks := make([]checkResult, 0, 2)
	for _, bin := range []string{"kind", "kubectl"} {
		checks = append(checks, checkKindTool(bin))
	}
	return checks
}

// checkKindTool reports a single kind/kubectl row, warning on absence,
// unreadable output, or a version below the pinned floor.
func checkKindTool(bin string) checkResult {
	if _, err := exec.LookPath(bin); err != nil {
		return checkResult{row: warnRow(bin,
			"not found — needed for local kubernetes bundles")}
	}

	out, err := kindToolVersion(bin)
	m := toolVersionPattern.FindStringSubmatch(out)
	if err != nil || m == nil {
		return checkResult{row: warnRow(bin, "version unreadable")}
	}

	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	floor := kindToolFloors[bin]
	if major < floor[0] || (major == floor[0] && minor < floor[1]) {
		return checkResult{row: warnRow(bin,
			fmt.Sprintf("%s found — below v%d.%d, kubernetes bundles may misbehave",
				m[0], floor[0], floor[1]))}
	}
	return checkResult{row: okRow(bin, m[0])}
}

// checkStateDirs verifies the XDG dirs are creatable; nil means healthy
// (the row only renders on failure).
func checkStateDirs() *checkResult {
	ok := true
	if cp, err := config.Path(); err != nil ||
		os.MkdirAll(filepath.Dir(cp), 0o755) != nil {
		ok = false
	}
	if r, err := session.Root(); err != nil || os.MkdirAll(r, 0o755) != nil {
		ok = false
	}
	if _, err := terraform.PluginCacheDir(); err != nil {
		ok = false
	}
	if ok {
		return nil
	}

	return &checkResult{row: failRow("state dirs", "cannot create XDG directories"),
		problem: true}
}

// credChecks reports one row per credential-bearing provider — VM, AI and
// access alike; anyConfigured feeds the NEXT decision.
func credChecks(ctx context.Context) ([]checkResult, bool) {
	cfg, err := config.Load()
	if err != nil {
		return []checkResult{{row: failRow("credentials",
			"config unreadable: "+err.Error()), problem: true}}, false
	}

	checks, anyConfigured := []checkResult{}, false
	for _, pr := range providers {
		v, isValidator := pr.(provider.CredentialValidator)
		if !isValidator {
			continue
		}
		if !v.Configured(cfg) {
			checks = append(checks, checkResult{row: warnRow(pr.Label(), "not configured")})
			continue
		}

		anyConfigured = true
		if err := validateCreds(ctx, v, cfg); err != nil {
			checks = append(checks, checkResult{row: failRow(pr.Label(), err.Error()),
				problem: true})
			continue
		}
		checks = append(checks, checkResult{row: okRow(pr.Label(), "credentials valid")})
	}
	return checks, anyConfigured
}
