package aws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
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

func ec2Client(t *testing.T, handler http.Handler) *ec2.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewEC2("k", "s", "eu-central-1", func(o *ec2.Options) {
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

func TestInstanceTypesFiltersAndSorts(t *testing.T) {
	c := ec2Client(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.Write([]byte(`<DescribeInstanceTypesResponse
  xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>x</requestId>
  <instanceTypeSet>
    <item><instanceType>m7i.2xlarge</instanceType>
      <vCpuInfo><defaultVCpus>8</defaultVCpus></vCpuInfo>
      <memoryInfo><sizeInMiB>32768</sizeInMiB></memoryInfo></item>
    <item><instanceType>c7i.xlarge</instanceType>
      <vCpuInfo><defaultVCpus>4</defaultVCpus></vCpuInfo>
      <memoryInfo><sizeInMiB>8192</sizeInMiB></memoryInfo></item>
    <item><instanceType>m7i.xlarge</instanceType>
      <vCpuInfo><defaultVCpus>4</defaultVCpus></vCpuInfo>
      <memoryInfo><sizeInMiB>16384</sizeInMiB></memoryInfo></item>
  </instanceTypeSet>
</DescribeInstanceTypesResponse>`))
	}))
	got, err := InstanceTypes(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}

	// c7i is not an m-family; m7i.xlarge (4 vCPU) sorts before m7i.2xlarge.
	if len(got) != 2 || got[0].Slug != "m7i.xlarge" || got[1].Slug != "m7i.2xlarge" {
		t.Fatalf("InstanceTypes() = %+v", got)
	}
	want := "m7i.xlarge  4 vCPU, 16 GiB"
	if got[0].Label != want {
		t.Fatalf("label = %q, want %q", got[0].Label, want)
	}
}
