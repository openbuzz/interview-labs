package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/stack"
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

// stackProfiles are the buildable vscode stacks, menu order. Bake target
// names equal profile names.
var stackProfiles = []string{"backend", "devops", "backend-ai", "devops-ai"}

// isAIProfile reports whether a profile bakes the AI CLIs (and so mints a key).
func isAIProfile(p string) bool { return strings.HasSuffix(p, "-ai") }

// pickProfile is a seam; production shows a huh select over the profiles.
var pickProfile = func(preselect string) (string, error) {
	desc := map[string]string{
		"backend":    "Go, Node and Python toolchains",
		"devops":     "terraform, kubectl, helm, aws-cli and friends",
		"backend-ai": "backend plus Claude Code, Codex and OpenCode",
		"devops-ai":  "devops plus Claude Code, Codex and OpenCode",
	}
	opts := make([]huh.Option[string], 0, len(stackProfiles))
	for _, p := range stackProfiles {
		opts = append(opts, huh.NewOption(fmt.Sprintf("%-12s— %s", p, desc[p]), p))
	}
	sel := preselect
	err := ui.SelectForm("Select a stack profile",
		"What the candidate's environment ships. -ai profiles add the AI CLIs.",
		opts, &sel)
	return sel, err
}

// ensureProfile resolves the stack profile: flag, else picker (TTY), else
// the devops default; the pick is remembered in config like roles.vm.
func ensureProfile(cfg *config.Config, flag string) (string, error) {
	p := flag
	switch {
	case p != "":
		if !slices.Contains(stackProfiles, p) {
			return "", usageError(fmt.Sprintf("unknown profile %q (one of %s)",
				p, strings.Join(stackProfiles, ", ")))
		}
	case !isTTY():
		p = "devops"
	default:
		pre := cfg.Profile
		if pre == "" {
			pre = "devops"
		}
		var err error
		if p, err = pickProfile(pre); err != nil {
			return "", err
		}
	}

	if p != cfg.Profile {
		cfg.Profile = p
		return p, cfg.Write()
	}
	return p, nil
}

// launchFlags bundles the launch command's flag values.
type launchFlags struct {
	region, size, profile string
	image, gateway, tag   string
	yes, noAI, noDNS      bool
}

func newLaunchCmd() *cobra.Command {
	f := &launchFlags{}
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "deploy a session VM",
		Long: `Deploy a fresh interview session.

Steps: pick a configured provider, a stack profile, a region and an
instance size (hourly price shown), then terraform applies the session,
cloud-init installs docker while the VM boots, and the CLI pulls and
starts the interview stack (gateway + vscode + dind) on it. The handover
prints the session URL and gateway password. Selections are remembered
and preselected next time. Pass --yes to skip the billing confirmation.
Sessions are independent and run in parallel.

Picking "local" runs the same stack on this machine's docker engine —
no cloud account, no billing, one local session at a time.

When OpenRouter is configured and an -ai profile is picked, launch mints
a spend-capped AI key for the stack; when Cloudflare is configured the
session gets a proxied DNS record. --no-ai and --no-dns skip them.

The VM bills hourly until "interview destroy".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLaunchCmd(cmd, f)
		},
	}
	cmd.Flags().StringVar(&f.region, "region", "",
		"provider region slug (e.g. fra1, fsn1, eu-central-1)")
	cmd.Flags().StringVar(&f.size, "size", "",
		"provider size slug (e.g. s-2vcpu-2gb, cx22, m7i.xlarge)")
	cmd.Flags().StringVar(&f.profile, "profile", "",
		"stack profile: backend, devops, backend-ai, devops-ai")
	cmd.Flags().BoolVar(&f.yes, "yes", false, "skip the billing confirmation prompt")
	cmd.Flags().BoolVar(&f.noAI, "no-ai", false, "skip the per-session AI key mint")
	cmd.Flags().BoolVar(&f.noDNS, "no-dns", false, "skip the per-session DNS record")
	cmd.Flags().StringVar(&f.image, "image", "",
		"vscode image ref override (full ref, used verbatim)")
	cmd.Flags().StringVar(&f.gateway, "gateway", "",
		"gateway image ref override (full ref, used verbatim)")
	cmd.Flags().StringVar(&f.tag, "tag", "",
		"image tag override; \"local\" uses task docker:build output")
	return cmd
}

// runLaunchCmd resolves provider and profile, then dispatches on whether the
// pick has the VM capability: cloud provider vs. the local pseudo-provider.
func runLaunchCmd(cmd *cobra.Command, f *launchFlags) error {
	out := cmd.OutOrStdout()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	printLogoOnce(out)
	printNarrowWarning(out)

	sel, err := selectVMProvider(out, &cfg)
	if err != nil {
		return err
	}
	profile, err := ensureProfile(&cfg, f.profile)
	if err != nil {
		return err
	}
	vm, ok := sel.(provider.VM)
	if !ok {
		return runLocalLaunch(cmd, out, cfg, sel, profile, f)
	}
	return runCloudLaunch(cmd, out, cfg, vm, profile, f)
}

// runCloudLaunch drives the terraform-backed pipeline.
func runCloudLaunch(cmd *cobra.Command, out io.Writer, cfg config.Config,
	vm provider.VM, profile string, f *launchFlags) error {
	images, err := resolveAndValidateImages(out, f, profile)
	if err != nil {
		return err
	}

	bin, err := terraform.Find()
	if err != nil {
		return err
	}
	regionLabel, si, err := ensureRegionSize(cmd.Context(), out, vm, &cfg,
		&f.region, &f.size)
	if err != nil {
		return err
	}
	cancelled, err := confirmGate(out, vm, regionLabel, f.region, si, f.size, f.yes)
	if err != nil || cancelled {
		return err
	}

	ai := activeAI(cfg, f.noAI || !isAIProfile(profile))
	access := activeAccess(cfg, f.noDNS)
	s, err := newLaunchSession(f.region, f.size, vm.Image(), vm.SSHUser(), profile,
		sessionRoles(vm, ai, access),
		session.TerraformInfo{Binary: bin.Name, Version: bin.Version})
	if err != nil {
		return err
	}
	release, err := s.Lock()
	if err != nil {
		return err
	}
	defer release()

	if err := runLaunch(cmd.Context(), out,
		buildLaunchInputs(cfg, vm, ai, access, images), bin, s); err != nil {
		return failLaunch(out, s, err)
	}
	return nil
}

// resolveAndValidateImages resolves the session's image refs and fails
// before any phase runs — refs land verbatim on the remote command line.
// Split out of runCloudLaunch to stay under the complexity gate.
func resolveAndValidateImages(out io.Writer, f *launchFlags,
	profile string) (resolvedImages, error) {
	images := resolveImages(f, profile)
	if err := images.validate(); err != nil {
		return resolvedImages{}, err
	}
	if images.Warning != "" {
		fmt.Fprintln(out, ui.RowWarn("images", images.Warning))
	}
	return images, nil
}

// newLaunchSession creates the session and mints its gateway password —
// split out of runCloudLaunch to stay under the complexity gate.
func newLaunchSession(region, size, image, sshUser, profile string,
	roles map[string]string, tf session.TerraformInfo) (*session.Session, error) {
	s, err := session.New(region, size, image, sshUser, roles, tf)
	if err != nil {
		return nil, err
	}
	pw, err := stack.GeneratePassword()
	if err != nil {
		return nil, err
	}
	s.Meta.Profile, s.Meta.GatewayPassword = profile, pw
	return s, s.Save()
}

// confirmGate prints the pre-provision summary on a TTY and, unless --yes,
// blocks on the billing confirm. cancelled reports a clean decline: RunE
// returns nil without provisioning.
func confirmGate(out io.Writer, vm provider.VM, regionLabel, region string,
	si *provider.SizeInfo, size string, yes bool) (cancelled bool, err error) {
	if !isTTY() {
		return false, nil
	}
	fmt.Fprintln(out, launchSummary(vm, regionLabel, region, si, size))
	fmt.Fprintln(out)
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

// selectVMProvider gates on configured vm-role providers, picks one on a
// TTY, and persists the pick as the session-role preselect.
func selectVMProvider(out io.Writer, cfg *config.Config) (provider.Provider, error) {
	configured := make([]provider.Provider, 0, len(providers))
	for _, p := range provider.ByRole(providers, provider.RoleVM) {
		if p.Configured(*cfg) {
			configured = append(configured, p)
		}
	}
	if len(configured) == 0 {
		fmt.Fprintln(out, ui.Section(ui.Fail.Render("NO PROVIDERS CONFIGURED"),
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

	cfg.Roles.VM = sel.Name()
	if err := cfg.Write(); err != nil {
		return nil, err
	}
	return sel, nil
}

// activeAI resolves the AI provider a launch uses: the sole configured one,
// unless disabled by flag. nil means the session runs without an AI key.
func activeAI(cfg config.Config, disabled bool) provider.AI {
	if disabled {
		return nil
	}
	for _, p := range provider.ByRole(providers, provider.RoleAI) {
		if ai, ok := p.(provider.AI); ok && p.Configured(cfg) {
			return ai
		}
	}
	return nil
}

// activeAccess resolves the access provider the same way.
func activeAccess(cfg config.Config, disabled bool) provider.Access {
	if disabled {
		return nil
	}
	for _, p := range provider.ByRole(providers, provider.RoleAccess) {
		if acc, ok := p.(provider.Access); ok && p.Configured(cfg) {
			return acc
		}
	}
	return nil
}

// sessionRoles records which provider serves each role this session.
func sessionRoles(vm provider.VM, ai provider.AI, access provider.Access) map[string]string {
	roles := map[string]string{"vm": vm.Name()}
	if ai != nil {
		roles["ai"] = ai.Name()
	}
	if access != nil {
		roles["access"] = access.Name()
	}
	return roles
}

// mergeCreds folds credential env maps left to right; key sets are disjoint
// by construction (each provider owns its vendor's env names).
func mergeCreds(maps ...map[string]string) map[string]string {
	out := map[string]string{}
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// launchInputs bundles what runLaunch needs beyond the session itself.
type launchInputs struct {
	cfg    config.Config
	creds  map[string]string
	tfvars map[string]any
	ai     provider.AI
	images resolvedImages
}

// buildLaunchInputs assembles creds, tfvars and image refs from the actives.
func buildLaunchInputs(cfg config.Config, vm provider.VM, ai provider.AI,
	access provider.Access, images resolvedImages) launchInputs {
	in := launchInputs{cfg: cfg, creds: vm.EnvCreds(cfg), ai: ai, images: images}
	if access != nil {
		in.creds = mergeCreds(in.creds, access.EnvCreds(cfg))
		in.tfvars = access.TFVars(cfg)
	}
	return in
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

// launchSummary renders the pre-provision summary; falls back to raw
// slugs (and no price row) when the flags path supplied no SizeInfo.
func launchSummary(vm provider.VM, regionLabel, region string,
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
	return ui.Section(ui.SectionTitle("launch"), rows...)
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
func runLaunch(ctx context.Context, out io.Writer, in launchInputs,
	bin terraform.Binary, s *session.Session) error {
	quiet := quietOutput()
	runnerOut := out
	if quiet {
		runnerOut = io.Discard
	}
	runner, err := newSessionRunner(runnerOut, in.creds, bin, s)
	if err != nil {
		return err
	}

	ip, err := applyPhases(ctx, out, quiet, runner, in, s)
	if err != nil {
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

	if err := stackPhases(ctx, out, quiet, client, in, s); err != nil {
		return err
	}
	return printLaunchSummary(out, s)
}

// applyPhases runs stage/terraform-init/terraform-apply and returns the VM
// address terraform emitted; split out of runLaunch to stay under the
// complexity gate.
func applyPhases(ctx context.Context, out io.Writer, quiet bool, runner *terraform.Runner,
	in launchInputs, s *session.Session) (ip string, err error) {
	if err := step(out, quiet, "stage", func() error {
		return stageSession(s, in.tfvars)
	}); err != nil {
		return "", err
	}
	if err := step(out, quiet, "terraform init", func() error {
		return tfInit(ctx, runner, s)
	}); err != nil {
		return "", err
	}
	if err := step(out, quiet, "terraform apply", func() error {
		if err := tfApply(ctx, runner, s); err != nil {
			return err
		}
		var ipErr error
		ip, ipErr = fetchIP(ctx, runner, s)
		return ipErr
	}); err != nil {
		return "", err
	}
	return ip, nil
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

// stackPhases provisions the container stack on the VM: docker readiness,
// compose push, image pull, optional key mint, compose up. Pull runs before
// mint so a failed pull never leaves an orphaned AI key.
func stackPhases(ctx context.Context, out io.Writer, quiet bool, client *ssh.Client,
	in launchInputs, s *session.Session) error {
	if err := step(out, quiet, "cloud-init", func() error {
		return waitCloudInit(ctx, client, s)
	}); err != nil {
		return err
	}
	if err := step(out, quiet, "push stack", func() error {
		return pushStack(ctx, client, s)
	}); err != nil {
		return err
	}
	if err := step(out, quiet, "pull stack", func() error {
		return pullStack(ctx, out, quiet, client, in.images, s)
	}); err != nil {
		return err
	}

	aiKey := ""
	if in.ai != nil {
		if err := step(out, quiet, "mint ai key", func() error {
			var mintErr error
			aiKey, mintErr = mintAIKey(ctx, in.ai, in.cfg, s)
			return mintErr
		}); err != nil {
			return err
		}
	}
	return step(out, quiet, "compose up", func() error {
		return upStack(ctx, client, in.images, s, aiKey)
	})
}

// waitCloudInit blocks until the boot-time docker install finishes, on its
// own budget on top of the launch context.
func waitCloudInit(ctx context.Context, client *ssh.Client, s *session.Session) error {
	if err := s.SetPhase("cloud-init"); err != nil {
		return err
	}
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	out, err := client.Run(waitCtx, "cloud-init status --wait")
	if err != nil {
		return fmt.Errorf("cloud-init failed — see /var/log/cloud-init-output.log "+
			"on the VM: %w (output: %s)", err, strings.TrimSpace(out))
	}
	return logWrite(s, "cloud-init.log", out)
}

// pushStack sends the staged compose file over ssh stdin into RemoteDir.
func pushStack(ctx context.Context, client *ssh.Client, s *session.Session) error {
	if err := s.SetPhase("push-stack"); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(s.StackDir(), "compose.yaml"))
	if err != nil {
		return err
	}

	out, err := client.RunIn(ctx, bytes.NewReader(data), stack.PushCmd())
	if err != nil {
		return fmt.Errorf("push stack: %w (output: %s)", err, strings.TrimSpace(out))
	}
	return nil
}

// pullStack pulls the session images on the VM — the slow phase; output
// streams to the session log (and the terminal when verbose).
func pullStack(ctx context.Context, out io.Writer, quiet bool, client *ssh.Client,
	images resolvedImages, s *session.Session) error {
	if err := s.SetPhase("pull-stack"); err != nil {
		return err
	}
	logPath := filepath.Join(s.LogsDir(), "stack-pull.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logFile.Close()

	dest := io.Writer(logFile)
	if !quiet {
		dest = io.MultiWriter(out, logFile)
	}
	cmd := stack.PullCmd(images.Gateway, images.Vscode)
	if err := client.RunStream(ctx, dest, cmd); err != nil {
		return fmt.Errorf("image pull failed (log: %s) — images for this build may "+
			"not be published yet; publish first or use --tag local with the "+
			"local provider: %w", logPath, err)
	}
	return nil
}

// upStack starts the stack; per-mode deltas cross as env sourced from stdin.
func upStack(ctx context.Context, client *ssh.Client, images resolvedImages,
	s *session.Session, aiKey string) error {
	if err := s.SetPhase("compose-up"); err != nil {
		return err
	}
	env := map[string]string{
		"GATEWAY_BIND":     "0.0.0.0:80",
		"GATEWAY_PASSWORD": s.Meta.GatewayPassword,
		"GATEWAY_IMAGE":    images.Gateway,
		"VSCODE_IMAGE":     images.Vscode,
	}
	if aiKey != "" {
		env["OPENROUTER_API_KEY"] = aiKey
	}

	out, err := client.RunIn(ctx, strings.NewReader(stack.EnvBlob(env)),
		stack.ComposeUpCmd(s.Meta.Slug))
	if err != nil {
		return fmt.Errorf("compose up failed: %w (output: %s)", err,
			strings.TrimSpace(out))
	}
	if err := logWrite(s, "stack-up.log", out); err != nil {
		return err
	}
	return s.SetURL(sessionURL(s))
}

// sessionURL prefers the DNS name when the session has one.
func sessionURL(s *session.Session) string {
	if s.Meta.FQDN != "" {
		return "http://" + s.Meta.FQDN
	}
	return "http://" + s.Meta.IP
}

// logWrite drops one phase's captured output into the session logs.
func logWrite(s *session.Session, name, data string) error {
	return os.WriteFile(filepath.Join(s.LogsDir(), name), []byte(data), 0o644)
}

// mintAIKey mints the session's spend-capped API key and persists the
// revoke handle immediately — a later phase failure must still revoke. The
// returned value stays in memory for compose up.
func mintAIKey(ctx context.Context, ai provider.AI, cfg config.Config,
	s *session.Session) (string, error) {
	if err := s.SetPhase("ai-mint"); err != nil {
		return "", err
	}
	key, err := ai.Mint(ctx, cfg, s.Meta.Slug)
	if err != nil {
		return "", err
	}
	s.Meta.AIKeyHash, s.Meta.AICapUSD = key.Hash, key.CapUSD
	return key.Key, s.Save()
}

// stageSession materializes terraform sources, the compose file, and
// tfvars into the session dir — including the cloud-init document the
// VM boots with.
func stageSession(s *session.Session, extra map[string]any) error {
	if err := s.SetPhase("stage"); err != nil {
		return err
	}
	if err := terraform.Stage(s.TerraformDir()); err != nil {
		return err
	}
	if err := stack.Stage(s.StackDir()); err != nil {
		return err
	}

	ud, err := stack.UserData(s.Meta.SSHUser)
	if err != nil {
		return err
	}
	vars := map[string]any{"user_data": ud}
	for k, v := range extra {
		vars[k] = v
	}
	return terraform.WriteTfvars(s.TerraformDir(), s.Meta.Roles["vm"], s.Meta.Region,
		s.Meta.Size, s.Meta.Image, s.Meta.Slug, s.SSHDir(), vars)
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

// fetchIP reads the root outputs and persists the address (and DNS name
// when the session has one).
func fetchIP(ctx context.Context, r *terraform.Runner,
	s *session.Session) (string, error) {
	if err := s.SetPhase("outputs"); err != nil {
		return "", err
	}
	outputs, err := r.Outputs(ctx)
	if err != nil {
		return "", err
	}
	if outputs.FQDN != "" {
		if err := s.SetFQDN(outputs.FQDN); err != nil {
			return "", err
		}
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
