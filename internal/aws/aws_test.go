package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/openbuzz/interview-labs/internal/provider"
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

	var m5Large *provider.SizeInfo
	for i, s := range got {
		if s.Slug == "m5.large" {
			m5Large = &got[i]
		}
		if s.MemGB < 4 || s.MemGB > 64 {
			t.Fatalf("%s MemGB = %d, want 4 <= MemGB <= 64", s.Slug, s.MemGB)
		}
	}
	if m5Large == nil {
		t.Fatal("m5.large not found in table")
	}
	want := provider.SizeInfo{
		Slug: "m5.large", Category: "General Purpose", VCPUs: 2, MemGB: 8,
		DiskGB: 40, Hourly: 0.096, Currency: "$",
	}
	if *m5Large != want {
		t.Fatalf("m5.large = %+v, want %+v", *m5Large, want)
	}
}

func TestInstanceTypesX86Only(t *testing.T) {
	for _, s := range InstanceTypes() {
		for _, arm := range []string{"m7g", "m8g", "t4g", "c7g"} {
			if strings.HasPrefix(s.Slug, arm+".") {
				t.Fatalf("ARM type offered: %s", s.Slug)
			}
		}
	}
}
