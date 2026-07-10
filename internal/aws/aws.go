// Package aws wraps the read-only AWS calls the CLI needs.
package aws

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/openbuzz/interview-labs/internal/provider"
)

// amiNameFilter finds Canonical's latest Ubuntu 26.04 LTS amd64 gp3 image;
// the terraform module's aws_ami data source consumes it as a name filter.
const amiNameFilter = "ubuntu/images/hvm-ssd-gp3/ubuntu-resolute-26.04-amd64-server-*"

// defaultRegion anchors region-agnostic calls (STS validation).
const defaultRegion = "eu-central-1"

// mFamilies gates the size picker to current-generation general-purpose
// Intel m-families (extend the list when AWS ships a successor).
var mFamilies = regexp.MustCompile(`^(m5|m6i|m7i|m8i)\.`)

// NewSTS builds an STS client from IAM user credentials (optFns allow a
// test endpoint).
func NewSTS(keyID, secret string, optFns ...func(*sts.Options)) *sts.Client {
	return sts.New(sts.Options{
		Region:      defaultRegion,
		Credentials: credentials.NewStaticCredentialsProvider(keyID, secret, ""),
	}, optFns...)
}

// NewEC2 builds an EC2 client for region from IAM user credentials.
func NewEC2(keyID, secret, region string, optFns ...func(*ec2.Options)) *ec2.Client {
	return ec2.New(ec2.Options{
		Region:      region,
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

// Regions is the curated region set: all four are enabled by default on
// every AWS account (no opt-in gating) and stock the m-families the size
// picker offers.
func Regions() []provider.Option {
	return []provider.Option{
		{Slug: "eu-central-1", Label: "eu-central-1  Frankfurt (EU)"},
		{Slug: "us-east-2", Label: "us-east-2  Ohio (US-east)"},
		{Slug: "us-west-2", Label: "us-west-2  Oregon (US-west)"},
		{Slug: "ap-southeast-1", Label: "ap-southeast-1  Singapore (Asia)"},
	}
}

// InstanceTypes lists current-generation general-purpose m-family types in
// the client's region, smallest first. AWS exposes no prices here.
func InstanceTypes(ctx context.Context, c *ec2.Client) ([]provider.Option, error) {
	type row struct {
		name   string
		vcpus  int32
		memMiB int64
	}
	var rows []row

	p := ec2.NewDescribeInstanceTypesPaginator(c, &ec2.DescribeInstanceTypesInput{
		Filters: []ec2types.Filter{{
			Name:   awssdk.String("current-generation"),
			Values: []string{"true"},
		}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, it := range page.InstanceTypes {
			name := string(it.InstanceType)
			if !mFamilies.MatchString(name) {
				continue
			}
			rows = append(rows, row{
				name:   name,
				vcpus:  awssdk.ToInt32(it.VCpuInfo.DefaultVCpus),
				memMiB: awssdk.ToInt64(it.MemoryInfo.SizeInMiB),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].vcpus != rows[j].vcpus {
			return rows[i].vcpus < rows[j].vcpus
		}
		if rows[i].memMiB != rows[j].memMiB {
			return rows[i].memMiB < rows[j].memMiB
		}
		return rows[i].name < rows[j].name
	})

	out := make([]provider.Option, 0, len(rows))
	for _, r := range rows {
		out = append(out, provider.Option{
			Slug: r.name,
			Label: fmt.Sprintf("%s  %d vCPU, %.0f GiB",
				r.name, r.vcpus, float64(r.memMiB)/1024),
		})
	}
	return out, nil
}
