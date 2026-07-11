// Package aws wraps the read-only AWS calls the CLI needs.
package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/openbuzz/interview-labs/internal/provider"
)

// amiNameFilter finds Canonical's latest Ubuntu 26.04 LTS amd64 gp3 image;
// the terraform module's aws_ami data source consumes it as a name filter.
const amiNameFilter = "ubuntu/images/hvm-ssd-gp3/ubuntu-resolute-26.04-amd64-server-*"

// defaultRegion anchors region-agnostic calls (STS validation).
const defaultRegion = "eu-central-1"

// NewSTS builds an STS client from IAM user credentials (optFns allow a
// test endpoint).
func NewSTS(keyID, secret string, optFns ...func(*sts.Options)) *sts.Client {
	return sts.New(sts.Options{
		Region:      defaultRegion,
		Credentials: credentials.NewStaticCredentialsProvider(keyID, secret, ""),
	}, optFns...)
}

// ValidateCreds verifies the credentials with a read-only identity fetch.
func ValidateCreds(ctx context.Context, c *sts.Client) error {
	_, err := c.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "InvalidClientTokenId", "SignatureDoesNotMatch", "AccessDenied":
				return fmt.Errorf("credentials rejected by AWS: %w", err)
			}
		}
		return fmt.Errorf("could not validate AWS credentials: %w", err)
	}
	return nil
}

// Regions is the curated global set: 1-2 per major geography, all enabled by
// default on every AWS account (no opt-in gating).
func Regions() []provider.Option {
	return []provider.Option{
		{Slug: "eu-central-1", Label: "eu-central-1  Frankfurt (EU)"},
		{Slug: "eu-west-1", Label: "eu-west-1  Ireland (EU)"},
		{Slug: "us-east-2", Label: "us-east-2  Ohio (US-East)"},
		{Slug: "us-west-2", Label: "us-west-2  Oregon (US-West)"},
		{Slug: "sa-east-1", Label: "sa-east-1  São Paulo (South America)"},
		{Slug: "ap-south-1", Label: "ap-south-1  Mumbai (Asia)"},
		{Slug: "ap-southeast-1", Label: "ap-southeast-1  Singapore (Asia)"},
		{Slug: "ap-northeast-1", Label: "ap-northeast-1  Tokyo (Asia)"},
	}
}

// familyBaseHourly is the on-demand Linux $/hr of <family>.large in us-east-1,
// from https://aws.amazon.com/ec2/pricing/on-demand/ as of 2026-07-11. One
// reference region by design — labels carry a tilde; sizes double the price
// per step. Update by hand when families or prices drift.
var familyBaseHourly = map[string]float64{
	"m5":  0.096,
	"m6i": 0.096,
	"m7i": 0.1008,
	"m8i": 0.10584,
}

// sizeSteps: size suffix, price multiplier vs .large, and the static shape.
var sizeSteps = []struct {
	suffix string
	mult   int
	vcpus  int
	memGiB int
}{
	{"large", 1, 2, 8},
	{"xlarge", 2, 4, 16},
	{"2xlarge", 4, 8, 32},
	{"4xlarge", 8, 16, 64},
}

// InstanceTypes is the curated x86 general-purpose menu (amd64-only
// policy) as a static table: no EC2 API call, no ec2:Describe* needed.
// DiskGB is the fixed gp3 root volume from terraform/aws/main.tf.
func InstanceTypes() []provider.SizeInfo {
	out := make([]provider.SizeInfo, 0, len(familyBaseHourly)*len(sizeSteps))
	for fam, base := range familyBaseHourly {
		for _, s := range sizeSteps {
			out = append(out, provider.SizeInfo{
				Slug:     fam + "." + s.suffix,
				Category: "General Purpose",
				VCPUs:    s.vcpus,
				MemGB:    s.memGiB,
				DiskGB:   40,
				Hourly:   base * float64(s.mult),
				Currency: "$",
			})
		}
	}
	return out
}
