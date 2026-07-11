package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// sshDialPort is a seam: production 22, tests point it at a local server.
var sshDialPort = 22

// pickRegionForm / pickSizeForm are seams; production runs huh selects.
var pickRegionForm = func(opts []huh.Option[string], preselect string) (string, error) {
	sel := preselect
	err := ui.SelectForm("Select a region",
		"Pick one geographically close to your candidate — lower latency for their shell.",
		opts, &sel)
	return sel, err
}

var pickSizeForm = func(opts []huh.Option[string], preselect string) (string, error) {
	sel := preselect
	err := ui.SelectForm("Select an instance size",
		"Hourly billing runs from launch until destroy. ESC goes back to region.",
		opts, &sel)
	return sel, err
}

// pickRegionSize is a seam; production loops region -> size, where ESC at
// the size step or an empty size list returns to the region step.
var pickRegionSize = func(ctx context.Context, out io.Writer, vm provider.VM,
	cfg config.Config) (provider.Option, provider.SizeInfo, error) {
	regions, err := vm.Regions(ctx, cfg)
	if err != nil {
		return provider.Option{}, provider.SizeInfo{}, err
	}
	regionOpts := make([]huh.Option[string], 0, len(regions))
	for _, r := range regions {
		regionOpts = append(regionOpts, huh.NewOption(r.Label, r.Slug))
	}
	defRegion, defSize := vm.Defaults(cfg)

	for {
		regionSlug, err := pickRegionForm(regionOpts, defRegion)
		if err != nil {
			return provider.Option{}, provider.SizeInfo{}, err
		}
		defRegion = regionSlug

		sizeOpts, bySlug, err := sizeOptions(ctx, vm, cfg, regionSlug)
		if err != nil {
			return provider.Option{}, provider.SizeInfo{}, err
		}
		if len(sizeOpts) == 0 {
			fmt.Fprintln(out, ui.RowWarn(regionSlug,
				"no matching sizes — pick another region"))
			continue
		}

		sizeSlug, err := pickSizeForm(sizeOpts, defSize)
		if errors.Is(err, huh.ErrUserAborted) {
			continue // ESC at size: back to the region step
		}
		if err != nil {
			return provider.Option{}, provider.SizeInfo{}, err
		}
		return regionBySlug(regions, regionSlug), bySlug[sizeSlug], nil
	}
}

// sizeOptions fetches, sorts (cheapest hourly, slug tie-break) and labels
// the sizes of one region.
func sizeOptions(ctx context.Context, vm provider.VM, cfg config.Config,
	region string) ([]huh.Option[string], map[string]provider.SizeInfo, error) {
	sizes, err := vm.Sizes(ctx, cfg, region)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(sizes, func(i, j int) bool {
		if sizes[i].Hourly != sizes[j].Hourly {
			return sizes[i].Hourly < sizes[j].Hourly
		}
		return sizes[i].Slug < sizes[j].Slug
	})

	opts := make([]huh.Option[string], 0, len(sizes))
	bySlug := make(map[string]provider.SizeInfo, len(sizes))
	for _, s := range sizes {
		opts = append(opts, huh.NewOption(ui.SizeLabel(s), s.Slug))
		bySlug[s.Slug] = s
	}
	return opts, bySlug, nil
}

// regionBySlug returns the picked region option (label included) by slug.
func regionBySlug(regions []provider.Option, slug string) provider.Option {
	for _, r := range regions {
		if r.Slug == slug {
			return r
		}
	}
	return provider.Option{Slug: slug, Label: slug}
}

// pickVMProvider is a seam; production shows a huh select over configured providers.
var pickVMProvider = func(configured []provider.Provider,
	preselect string) (provider.Provider, error) {
	opts := make([]huh.Option[string], 0, len(configured))
	for _, p := range configured {
		opts = append(opts, huh.NewOption(ui.Badge(true)+" "+p.Label(), p.Name()))
	}
	sel := preselect
	if err := ui.SelectForm("Select a cloud provider",
		"Hosts this session's VM. Only configured providers are listed.",
		opts, &sel); err != nil {
		return nil, err
	}
	for _, p := range configured {
		if p.Name() == sel {
			return p, nil
		}
	}
	return configured[0], nil
}

// confirmLaunch is a seam; production asks the billing gate, Yes focused.
var confirmLaunch = func() (bool, error) {
	ok := true
	err := ui.ConfirmForm(
		"Cloud resources will be provisioned — billing starts. Continue?", "", &ok)
	return ok, err
}

func newLaunchCmd() *cobra.Command {
	var region, size string
	var yes bool
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "deploy a session VM",
		Long: `Deploy a fresh interview VM.

Steps: pick a configured provider, a region and an instance size (hourly
price shown), then terraform applies the session, waits for ssh and
prints a ready-to-paste ssh line. Selections are remembered and
preselected next time. Pass --yes to skip the billing confirmation.
Sessions are independent and run in parallel.

The VM bills hourly until "interview destroy".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunchCmd(cmd, &region, &size, yes)
		},
	}
	cmd.Flags().StringVar(&region, "region", "",
		"provider region slug (e.g. fra1, fsn1, eu-central-1)")
	cmd.Flags().StringVar(&size, "size", "",
		"provider size slug (e.g. s-2vcpu-2gb, cx22, m7i.xlarge)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip the billing confirmation prompt")
	return cmd
}

// runLaunchCmd resolves a provider, region and size, gates on the billing
// confirm, and drives the launch.
func runLaunchCmd(cmd *cobra.Command, region, size *string, yes bool) error {
	out := cmd.OutOrStdout()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	printLogoOnce(out)
	printNarrowWarning(out)

	vm, err := selectVMProvider(out, &cfg)
	if err != nil {
		return err
	}
	bin, err := terraform.Find()
	if err != nil {
		return err
	}
	regionLabel, si, err := ensureRegionSize(cmd.Context(), out, vm, &cfg, region, size)
	if err != nil {
		return err
	}
	cancelled, err := confirmGate(out, vm, regionLabel, *region, si, *size, yes)
	if err != nil {
		return err
	}
	if cancelled {
		return nil
	}

	s, err := session.New(*region, *size, vm.Image(), vm.SSHUser(),
		map[string]string{"vm": vm.Name()},
		session.TerraformInfo{Binary: bin.Name, Version: bin.Version})
	if err != nil {
		return err
	}
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()

	if err := runLaunch(cmd.Context(), out, vm.EnvCreds(cfg), bin, s); err != nil {
		return failLaunch(out, s, err)
	}
	return nil
}

// confirmGate prints the pre-provision summary on a TTY and, unless --yes,
// blocks on the billing confirm. cancelled reports a clean decline: RunE
// returns nil without provisioning.
func confirmGate(out io.Writer, vm provider.VM, regionLabel, region string,
	si *provider.SizeInfo, size string, yes bool) (cancelled bool, err error) {
	if !isTTY() {
		return false, nil
	}
	fmt.Fprintln(out, launchSummaryBox(vm, regionLabel, region, si, size))
	if yes {
		return false, nil
	}
	ok, err := confirmLaunch()
	if err != nil {
		return false, err
	}
	if !ok {
		fmt.Fprintln(out, "launch cancelled — nothing provisioned")
		return true, nil
	}
	return false, nil
}

// selectVMProvider gates on configured providers, picks one on a TTY, and
// persists the pick as the session-role preselect.
func selectVMProvider(out io.Writer, cfg *config.Config) (provider.VM, error) {
	configured := make([]provider.Provider, 0, len(providers))
	for _, p := range provider.ByRole(providers, provider.RoleVM) {
		if p.Configured(*cfg) {
			configured = append(configured, p)
		}
	}
	if len(configured) == 0 {
		fmt.Fprintln(out, ui.Box("No providers configured", ui.Fail,
			"Launch needs a configured cloud provider to host the interview VM.",
			"",
			`Run "interview init" to configure one, then re-run "interview launch".`))
		return nil, fmt.Errorf("no providers configured")
	}

	sel := configured[0]
	if isTTY() {
		var err error
		sel, err = pickVMProvider(configured, cfg.Roles.VM)
		if err != nil {
			return nil, err
		}
	}
	vm, ok := sel.(provider.VM)
	if !ok {
		return nil, fmt.Errorf("provider %s cannot host a session VM", sel.Name())
	}

	cfg.Roles.VM = vm.Name()
	if err := cfg.Write(); err != nil {
		return nil, err
	}
	return vm, nil
}

// ensureRegionSize fills missing region/size from the interactive loop and
// persists the picks; regionLabel and si are zero on the flags path.
func ensureRegionSize(ctx context.Context, out io.Writer, vm provider.VM,
	cfg *config.Config, region, size *string) (string, *provider.SizeInfo, error) {
	if *region != "" && *size != "" {
		return "", nil, nil
	}
	if !isTTY() {
		return "", nil, usageError("launch needs --region and --size when not on a terminal")
	}

	r, si, err := pickRegionSize(ctx, out, vm, *cfg)
	if err != nil {
		return "", nil, err
	}
	*region, *size = r.Slug, si.Slug
	vm.SetDefaults(cfg, r.Slug, si.Slug)
	return r.Label, &si, cfg.Write()
}

// launchSummaryBox renders the pre-provision summary; falls back to raw
// slugs (and no price row) when the flags path supplied no SizeInfo.
func launchSummaryBox(vm provider.VM, regionLabel, region string,
	si *provider.SizeInfo, size string) string {
	regionRow := regionLabel
	if regionRow == "" {
		regionRow = region
	}
	sizeRow, rows := size, []string{}
	if si != nil {
		sizeRow = fmt.Sprintf("%s — %d vCPU, %d GB memory, %d GB disk",
			si.Category, si.VCPUs, si.MemGB, si.DiskGB)
	}
	rows = append(rows,
		"Provider   "+vm.Label(),
		"Region     "+regionRow,
		"Size       "+sizeRow)
	if si != nil {
		price := math.Ceil(si.Hourly*100) / 100
		rows = append(rows, fmt.Sprintf("Price      ~%s%.2f/h, billed until %q",
			si.Currency, price, "interview destroy"))
	}
	return ui.Box("Launch summary", ui.Accent, rows...)
}

// step runs one launch/destroy phase: a spinner row when quiet, plain
// passthrough when verbose.
func step(out io.Writer, quiet bool, title string, fn func() error) error {
	if !quiet {
		return fn()
	}
	return ui.Step(out, title, func(func(string)) error { return fn() })
}

// runLaunch drives the launch phases; quiet mode renders each as a step row
// and keeps terraform output in the session logs only.
func runLaunch(ctx context.Context, out io.Writer,
	creds map[string]string, bin terraform.Binary, s *session.Session) error {
	quiet := quietOutput()
	runnerOut := out
	if quiet {
		runnerOut = io.Discard
	}
	runner, err := newSessionRunner(runnerOut, creds, bin, s)
	if err != nil {
		return err
	}

	if err := step(out, quiet, "stage", func() error {
		return stageSession(s)
	}); err != nil {
		return err
	}
	if err := step(out, quiet, "terraform init", func() error {
		return tfInit(ctx, runner, s)
	}); err != nil {
		return err
	}
	var ip string
	if err := step(out, quiet, "terraform apply", func() error {
		if err := tfApply(ctx, runner, s); err != nil {
			return err
		}
		var ipErr error
		ip, ipErr = fetchIP(ctx, runner, s)
		return ipErr
	}); err != nil {
		return err
	}

	var client *ssh.Client
	if err := step(out, quiet, "wait-ssh", func() error {
		var dialErr error
		client, dialErr = waitSSH(ctx, s, ip)
		return dialErr
	}); err != nil {
		return err
	}
	defer client.Close()

	if err := greet(ctx, out, client, s); err != nil {
		return err
	}
	return printLaunchSummary(out, s)
}

// newSessionRunner builds the terraform runner over the session dirs.
func newSessionRunner(out io.Writer, creds map[string]string,
	bin terraform.Binary, s *session.Session) (*terraform.Runner, error) {
	cache, err := terraform.PluginCacheDir()
	if err != nil {
		return nil, err
	}
	return &terraform.Runner{
		Bin: bin, Dir: s.TerraformDir(),
		Env: terraform.RunEnv(creds, cache), LogsDir: s.LogsDir(), Out: out,
	}, nil
}

// stageSession materializes the embedded tree and tfvars into the session dir.
func stageSession(s *session.Session) error {
	if err := s.SetPhase("stage"); err != nil {
		return err
	}
	if err := terraform.Stage(s.TerraformDir()); err != nil {
		return err
	}
	return terraform.WriteTfvars(s.TerraformDir(), s.Meta.Roles["vm"], s.Meta.Region,
		s.Meta.Size, s.Meta.Image, s.Meta.Slug, s.SSHDir())
}

func tfInit(ctx context.Context, r *terraform.Runner, s *session.Session) error {
	if err := s.SetPhase("terraform-init"); err != nil {
		return err
	}
	return r.Init(ctx)
}

func tfApply(ctx context.Context, r *terraform.Runner, s *session.Session) error {
	if err := s.SetPhase("terraform-apply"); err != nil {
		return err
	}
	return r.Apply(ctx)
}

// fetchIP reads the root outputs and persists the address.
func fetchIP(ctx context.Context, r *terraform.Runner,
	s *session.Session) (string, error) {
	if err := s.SetPhase("outputs"); err != nil {
		return "", err
	}
	outputs, err := r.Outputs(ctx)
	if err != nil {
		return "", err
	}
	return outputs.IP, s.SetIP(outputs.IP)
}

// waitSSH dials until the VM answers or the 5-minute budget expires.
func waitSSH(ctx context.Context, s *session.Session, ip string) (*ssh.Client, error) {
	if err := s.SetPhase("wait-ssh"); err != nil {
		return nil, err
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	addr := net.JoinHostPort(ip, strconv.Itoa(sshDialPort))
	return ssh.Dial(dialCtx, addr, s.Meta.SSHUser, s.KeyPath(), s.KnownHostsPath())
}

// greet runs the proof-of-life command and echoes its output.
func greet(ctx context.Context, out io.Writer, client *ssh.Client,
	s *session.Session) error {
	if err := s.SetPhase("hello"); err != nil {
		return err
	}
	hello, err := client.Run(ctx, "echo 'Hello world'")
	if err != nil {
		return err
	}
	fmt.Fprint(out, hello)
	return nil
}

// printLaunchSummary marks the session ready and prints the handover.
func printLaunchSummary(out io.Writer, s *session.Session) error {
	if err := s.SetPhase("summary"); err != nil {
		return err
	}
	if err := s.SetStatus(session.StatusReady); err != nil {
		return err
	}

	fmt.Fprintln(out)
	printHandover(out, s)
	return nil
}

// failLaunch records the failure and prints the recovery hints.
func failLaunch(out io.Writer, s *session.Session, err error) error {
	s.SetStatus(session.StatusFailed)
	fmt.Fprintf(out, "\n%s\n", ui.RowFail("launch "+s.Meta.Phase, err.Error()))
	fmt.Fprintf(out, "%s\n", ui.Faint.Render("logs: "+s.LogsDir()))
	fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
	return fmt.Errorf("launch failed in phase %s", s.Meta.Phase)
}
