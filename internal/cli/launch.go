package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
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

// pickRegionSize is a seam: production drives huh selects over the
// provider's live region and size lists.
var pickRegionSize = func(ctx context.Context, vm provider.VM,
	cfg config.Config) (string, string, error) {
	regions, err := vm.Regions(ctx, cfg)
	if err != nil {
		return "", "", err
	}

	region, size := vm.Defaults(cfg)
	regionOpts := make([]huh.Option[string], 0, len(regions))
	for _, r := range regions {
		regionOpts = append(regionOpts, huh.NewOption(r.Label, r.Slug))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Region").Options(regionOpts...).Value(&region),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return "", "", err
	}

	sizes, err := vm.Sizes(ctx, cfg, region)
	if err != nil {
		return "", "", err
	}
	sizeOpts := make([]huh.Option[string], 0, len(sizes))
	for _, s := range sizes {
		sizeOpts = append(sizeOpts, huh.NewOption(s.Label, s.Slug))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Size").Options(sizeOpts...).Value(&size),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return "", "", err
	}
	return region, size, nil
}

// pickVMProvider is a seam; production shows a huh select over configured providers.
var pickVMProvider = func(configured []provider.Provider,
	preselect string) (provider.Provider, error) {
	opts := make([]huh.Option[string], 0, len(configured))
	for _, p := range configured {
		opts = append(opts, huh.NewOption(ui.Badge(true)+" "+p.Label(), p.Name()))
	}
	sel := preselect
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Provider").Options(opts...).Value(&sel),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return nil, err
	}
	for _, p := range configured {
		if p.Name() == sel {
			return p, nil
		}
	}
	return configured[0], nil
}

func newLaunchCmd() *cobra.Command {
	var region, size string
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "deploy a session VM",
		Long: `Deploy a fresh interview VM.

Steps: pick a configured provider, a region and an instance size (hourly
price shown), then terraform applies the session, waits for ssh and
prints a ready-to-paste ssh line. Selections are remembered and
preselected next time. Sessions are independent and run in parallel.

The VM bills hourly until "interview destroy".`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			fmt.Fprintln(out, ui.Logo())

			configured := make([]provider.Provider, 0, len(providers))
			for _, p := range provider.ByRole(providers, provider.RoleVM) {
				if p.Configured(cfg) {
					configured = append(configured, p)
				}
			}
			if len(configured) == 0 {
				fmt.Fprintln(out, ui.Box("No providers configured", ui.Fail,
					"Launch needs a configured cloud provider to host the interview VM.",
					"",
					`Run "interview init" to configure one, then re-run "interview launch".`))
				return fmt.Errorf("no providers configured")
			}

			vmSel := configured[0]
			if isTTY() {
				vmSel, err = pickVMProvider(configured, cfg.Roles.VM)
				if err != nil {
					return err
				}
			}
			vm, ok := vmSel.(provider.VM)
			if !ok {
				return fmt.Errorf("provider %s cannot host a session VM", vmSel.Name())
			}
			cfg.Roles.VM = vm.Name()
			if err := cfg.Write(); err != nil {
				return err
			}

			bin, err := terraform.Find()
			if err != nil {
				return err
			}

			if region == "" || size == "" {
				if !isTTY() {
					return usageError(
						"launch needs --region and --size when not on a terminal")
				}
				region, size, err = pickRegionSize(cmd.Context(), vm, cfg)
				if err != nil {
					return err
				}
				vm.SetDefaults(&cfg, region, size)
				if err := cfg.Write(); err != nil {
					return err
				}
			}

			s, err := session.New(region, size, vm.Image(), vm.SSHUser(),
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
				s.SetStatus(session.StatusFailed)
				fmt.Fprintf(out, "\n%s\n", ui.RowFail("launch "+s.Meta.Phase, err.Error()))
				fmt.Fprintf(out, "%s\n", ui.Faint.Render("logs: "+s.LogsDir()))
				fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
				return fmt.Errorf("launch failed in phase %s", s.Meta.Phase)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&region, "region", "",
		"provider region slug (e.g. fra1, fsn1, eu-central-1)")
	cmd.Flags().StringVar(&size, "size", "",
		"provider size slug (e.g. s-2vcpu-2gb, cx22, m7i.xlarge)")
	return cmd
}

func runLaunch(ctx context.Context, out io.Writer,
	creds map[string]string, bin terraform.Binary, s *session.Session) error {
	cache, err := terraform.PluginCacheDir()
	if err != nil {
		return err
	}
	runner := &terraform.Runner{
		Bin: bin, Dir: s.TerraformDir(),
		Env: terraform.RunEnv(creds, cache), LogsDir: s.LogsDir(), Out: out,
	}

	if err := s.SetPhase("stage"); err != nil {
		return err
	}
	if err := terraform.Stage(s.TerraformDir()); err != nil {
		return err
	}
	if err := terraform.WriteTfvars(s.TerraformDir(), s.Meta.Roles["vm"], s.Meta.Region,
		s.Meta.Size, s.Meta.Image, s.Meta.Slug, s.SSHDir()); err != nil {
		return err
	}

	if err := s.SetPhase("terraform-init"); err != nil {
		return err
	}
	if err := runner.Init(ctx); err != nil {
		return err
	}
	if err := s.SetPhase("terraform-apply"); err != nil {
		return err
	}
	if err := runner.Apply(ctx); err != nil {
		return err
	}

	if err := s.SetPhase("outputs"); err != nil {
		return err
	}
	outputs, err := runner.Outputs(ctx)
	if err != nil {
		return err
	}
	if err := s.SetIP(outputs.IP); err != nil {
		return err
	}

	if err := s.SetPhase("wait-ssh"); err != nil {
		return err
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	addr := net.JoinHostPort(outputs.IP, strconv.Itoa(sshDialPort))
	client, err := ssh.Dial(dialCtx, addr, s.Meta.SSHUser, s.KeyPath(), s.KnownHostsPath())
	if err != nil {
		return err
	}
	defer client.Close()

	if err := s.SetPhase("hello"); err != nil {
		return err
	}
	hello, err := client.Run(ctx, "echo 'Hello world'")
	if err != nil {
		return err
	}
	fmt.Fprint(out, hello)

	if err := s.SetPhase("summary"); err != nil {
		return err
	}
	if err := s.SetStatus(session.StatusReady); err != nil {
		return err
	}
	sshLine := strings.Join(
		ssh.Argv(s.KeyPath(), s.KnownHostsPath(), s.Meta.SSHUser, outputs.IP), " ")
	fmt.Fprintf(out, "\n%s\n", ui.RowOK(s.Meta.Slug, outputs.IP))
	fmt.Fprintf(out, "%s\n", ui.Faint.Render(sshLine))
	fmt.Fprintln(out, ui.Next(
		"interview ssh "+s.Meta.Slug, "interview destroy "+s.Meta.Slug))
	return nil
}
