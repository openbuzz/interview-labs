package aws

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

func stsClient(t *testing.T, handler http.Handler) *sts.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewSTS("k", "s", func(o *sts.Options) {
		o.BaseEndpoint = awssdk.String(srv.URL)
	})
}

func TestValidateCredsOK(t *testing.T) {
	c := stsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<GetCallerIdentityResponse
  xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult>
    <Arn>arn:aws:iam::123456789012:user/interview-labs</Arn>
    <UserId>AIDAEXAMPLE</UserId>
    <Account>123456789012</Account>
  </GetCallerIdentityResult>
  <ResponseMetadata><RequestId>x</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`))
	}))
	if err := ValidateCreds(context.Background(), c); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCredsRejected(t *testing.T) {
	c := stsClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`<ErrorResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <Error><Type>Sender</Type><Code>InvalidClientTokenId</Code>
  <Message>The security token included in the request is invalid.</Message></Error>
  <RequestId>x</RequestId>
</ErrorResponse>`))
	}))
	err := ValidateCreds(context.Background(), c)
	if err == nil || !strings.Contains(err.Error(), "credentials rejected") {
		t.Fatalf("err = %v, want credentials rejected", err)
	}
}

func TestInstanceTypesStaticTable(t *testing.T) {
	got := InstanceTypes()

	if len(got) != 16 {
		t.Fatalf("len = %d, want 16 (4 families x 4 sizes)", len(got))
	}

	prices := map[string]float64{}
	for _, o := range got {
		var vcpu, mem int
		var price float64
		var name string
		_, err := fmt.Sscanf(o.Label, "%s %dvcpu %dGB ~$%f/hr", &name, &vcpu, &mem, &price)
		if err != nil || name != o.Slug {
			t.Fatalf("label %q does not parse: %v", o.Label, err)
		}
		prices[o.Slug] = price
	}

	// Labels round to whole cents (~$%.2f), so family doubling is asserted
	// within the compounded rounding tolerance (8x half-cent = 4c).
	if math.Abs(prices["m7i.xlarge"]-2*prices["m7i.large"]) > 0.05 ||
		math.Abs(prices["m7i.4xlarge"]-8*prices["m7i.large"]) > 0.05 {
		t.Fatalf("family prices not doubling: %v", prices)
	}

	for i := 1; i < len(got); i++ {
		if prices[got[i].Slug] < prices[got[i-1].Slug] {
			t.Fatalf("not sorted cheapest first at %s", got[i].Slug)
		}
	}
}

func TestInstanceTypesX86Only(t *testing.T) {
	for _, o := range InstanceTypes() {
		for _, arm := range []string{"m7g", "m8g", "t4g", "c7g"} {
			if strings.HasPrefix(o.Slug, arm+".") {
				t.Fatalf("ARM type offered: %s", o.Slug)
			}
		}
	}
}
