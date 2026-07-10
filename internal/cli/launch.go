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
	"github.com/openbuzz/interview-labs/internal/digitalocean"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// sshDialPort is a seam: production 22, tests point it at a local server.
var sshDialPort = 22

func sizeLabel(s digitalocean.Size) string {
	return fmt.Sprintf("%s  %dvcpu %dMB %dGB  $%.3f/hr ($%.0f/mo)",
		s.Slug, s.VCPUs, s.Memory, s.Disk, s.PriceHourly, s.PriceMonthly)
}

// pickRegionSize is a seam: production drives huh selects over live godo lists.
var pickRegionSize = func(ctx context.Context, token,
	preRegion, preInstance string) (string, string, error) {
	client, err := digitalocean.NewClient(token)
	if err != nil {
		return "", "", err
	}
	regions, err := digitalocean.Regions(ctx, client)
	if err != nil {
		return "", "", err
	}

	region := preRegion
	regionOpts := make([]huh.Option[string], 0, len(regions))
	for _, r := range regions {
		regionOpts = append(regionOpts, huh.NewOption(r.Slug+"  "+r.Name, r.Slug))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Region").Options(regionOpts...).Value(&region),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return "", "", err
	}

	sizes, err := digitalocean.SizesFor(ctx, client, region)
	if err != nil {
		return "", "", err
	}
	size := preInstance
	sizeOpts := make([]huh.Option[string], 0, len(sizes))
	for _, s := range sizes {
		sizeOpts = append(sizeOpts, huh.NewOption(sizeLabel(s), s.Slug))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Size (cheapest first)").
			Options(sizeOpts...).Value(&size),
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
		Short: "deploy a session VM on DigitalOcean",
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

			vm := configured[0]
			if isTTY() {
				vm, err = pickVMProvider(configured, cfg.Roles.VM)
				if err != nil {
					return err
				}
			}
			cfg.Roles.VM = vm.Name()
			if err := cfg.Write(); err != nil {
				return err
			}

			token := cfg.Token()
			if token == "" {
				return fmt.Errorf("no DigitalOcean token — run interview init first")
			}
			bin, err := terraform.Find()
			if err != nil {
				return err
			}

			if region == "" || size == "" {
				if !isTTY() {
					return usageError("launch needs --region and --size when not on a terminal")
				}
				region, size, err = pickRegionSize(cmd.Context(), token,
					cfg.Providers.DigitalOcean.Region, cfg.Providers.DigitalOcean.Instance)
				if err != nil {
					return err
				}
				cfg.Providers.DigitalOcean.Region = region
				cfg.Providers.DigitalOcean.Instance = size
				if err := cfg.Write(); err != nil {
					return err
				}
			}

			s, err := session.New(region, size, digitalocean.Image,
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

			if err := runLaunch(cmd.Context(), out, token, bin, s); err != nil {
				s.SetStatus(session.StatusFailed)
				fmt.Fprintf(out, "\n%s\n", ui.RowFail("launch "+s.Meta.Phase, err.Error()))
				fmt.Fprintf(out, "%s\n", ui.Faint.Render("logs: "+s.LogsDir()))
				fmt.Fprintln(out, ui.Next("interview destroy "+s.Meta.Slug))
				return fmt.Errorf("launch failed in phase %s", s.Meta.Phase)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&region, "region", "", "DigitalOcean region slug")
	cmd.Flags().StringVar(&size, "size", "", "DigitalOcean size slug")
	return cmd
}

func runLaunch(ctx context.Context, out io.Writer,
	token string, bin terraform.Binary, s *session.Session) error {
	cache, err := terraform.PluginCacheDir()
	if err != nil {
		return err
	}
	runner := &terraform.Runner{
		Bin: bin, Dir: s.TerraformDir(),
		Env: terraform.RunEnv(token, cache), LogsDir: s.LogsDir(), Out: out,
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
	client, err := ssh.Dial(dialCtx, addr, "root", s.KeyPath(), s.KnownHostsPath())
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
		ssh.Argv(s.KeyPath(), s.KnownHostsPath(), "root", outputs.IP), " ")
	fmt.Fprintf(out, "\n%s\n", ui.RowOK(s.Meta.Slug, outputs.IP))
	fmt.Fprintf(out, "%s\n", ui.Faint.Render(sshLine))
	fmt.Fprintln(out, ui.Next(
		"interview ssh "+s.Meta.Slug, "interview destroy "+s.Meta.Slug))
	return nil
}
