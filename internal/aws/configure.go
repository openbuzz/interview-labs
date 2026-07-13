package aws

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/openbuzz/interview-labs/internal/config"
	"github.com/openbuzz/interview-labs/internal/provider"
	"github.com/openbuzz/interview-labs/internal/ui"
)

const guidanceTitle = "How to create AWS credentials"

const guidance = `If you don't have an account yet, create one at:
  https://signin.aws.amazon.com/signup?request_type=register

1. Open https://console.aws.amazon.com/iam
2. Go to "Users" → "Create user"
3. Enter a name (e.g. "interview-labs"), click "Next"
4. Select "Attach policies directly", check "AdministratorAccess"
5. Click "Next" → "Create user"
6. Open the created user, go to "Security credentials" → "Create access key"
7. Select "Command Line Interface (CLI)", check the "I understand" box
8. Click "Next" → "Create access key"
9. Copy the Access Key ID and Secret Access Key
10. Paste the credentials into the prompts below`

// Seams for tests.
var (
	out io.Writer = os.Stdout

	promptCreds = func() (string, string, error) {
		var keyID, secret string
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title("AWS access key ID").
				EchoMode(huh.EchoModePassword).
				Validate(notEmpty("access key ID")).
				Value(&keyID),
			huh.NewInput().
				Title("AWS secret access key").
				EchoMode(huh.EchoModePassword).
				Validate(notEmpty("secret access key")).
				Value(&secret),
		)).WithTheme(ui.Theme()).WithKeyMap(ui.FormKeyMap())
		if err := form.Run(); err != nil {
			return "", "", err
		}
		return keyID, secret, nil
	}

	validatePair = func(ctx context.Context, keyID, secret string) error {
		return ValidateCreds(ctx, NewSTS(keyID, secret))
	}
)

func notEmpty(what string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is empty", what)
		}
		return nil
	}
}

type aw struct{}

// New returns the AWS provider.
func New() provider.Provider { return aw{} }

func (aw) Name() string  { return "aws" }
func (aw) Label() string { return "AWS" }

func (aw) Roles() []provider.Role { return []provider.Role{provider.RoleVM} }

func (aw) Configured(cfg config.Config) bool {
	id, secret := creds(cfg)
	return id != "" && secret != ""
}

// Configure shows the IAM guidance, prompts for the credential pair,
// validates it with retries, stores it.
func (aw) Configure(ctx context.Context, cfg *config.Config) error {
	fmt.Fprintln(out, ui.Section(ui.SectionTitle(guidanceTitle),
		strings.Split(guidance, "\n")...))
	fmt.Fprintln(out, ui.Faint.Render(
		"The IAM user credentials are validated before they are stored (0600)."))

	keyID, secret, err := promptCreds()
	if err != nil {
		return err
	}

	if err := provider.TestCredentials(ctx, out, ui.Step, func(ctx context.Context) error {
		return validatePair(ctx, keyID, secret)
	}); err != nil {
		fmt.Fprintln(out, ui.RowFail("credentials",
			"credentials rejected — nothing stored"))
		return nil
	}

	cfg.Providers.AWS.AccessKeyID = keyID
	cfg.Providers.AWS.SecretAccessKey = secret
	fmt.Fprintln(out, ui.RowOK("credentials", "valid — IAM user credentials stored"))
	return nil
}
