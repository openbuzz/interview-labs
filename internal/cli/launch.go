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
	"github.com/openbuzz/interview-labs/internal/session"
	"github.com/openbuzz/interview-labs/internal/ssh"
	"github.com/openbuzz/interview-labs/internal/terraform"
	"github.com/openbuzz/interview-labs/internal/ui"
)

// sshDialPort is a seam: production 22, tests point it at a local server.
var sshDialPort = 22

// pickRegionSize is a seam: production drives huh selects over live godo lists.
var pickRegionSize = func(ctx context.Context, token string) (string, string, error) {
	client, err := digitalocean.NewClient(token)
	if err != nil {
		return "", "", err
	}
	regions, err := digitalocean.Regions(ctx, client)
	if err != nil {
		return "", "", err
	}

	var region string
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
	var size string
	sizeOpts := make([]huh.Option[string], 0, len(sizes))
	for _, s := range sizes {
		label := fmt.Sprintf("%s  %dvcpu %dMB  $%.0f/mo",
			s.Slug, s.VCPUs, s.Memory, s.PriceMonthly)
		sizeOpts = append(sizeOpts, huh.NewOption(label, s.Slug))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Size (cheapest first)").
			Options(sizeOpts...).Value(&size),
	)).WithTheme(ui.Theme()).Run(); err != nil {
		return "", "", err
	}
	return region, size, nil
}

func newLaunchCmd() *cobra.Command {
	var region, size string
	cmd := &cobra.Command{
		Use:   "launch",
		Short: "deploy a session VM on DigitalOcean",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			cfg, err := config.Load()
			if err != nil {
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
				region, size, err = pickRegionSize(cmd.Context(), token)
				if err != nil {
					return err
				}
			}

			s, err := session.New(region, size, terraform.Image,
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
	if err := terraform.WriteTfvars(s.TerraformDir(),
		s.Meta.Region, s.Meta.Size, s.Meta.Image, s.Meta.Slug); err != nil {
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
	if err := ssh.WriteKeyFiles(s.SSHDir(),
		outputs.SSHPrivateKey, outputs.SSHPublicKey); err != nil {
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
