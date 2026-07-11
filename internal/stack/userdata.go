package stack

import (
	"strings"
	"text/template"
)

// userDataTmpl is the cloud-init document every session VM boots with:
// Docker's apt repo and engine install during boot (overlapped with
// terraform apply and the ssh wait), plus the payload dir owned by the
// ssh user. Steps mirror docs.docker.com/engine/install/ubuntu.
const userDataTmpl = `#cloud-config
package_update: true
packages:
  - ca-certificates
  - curl
runcmd:
  - install -m 0755 -d /etc/apt/keyrings
  - curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  - chmod a+r /etc/apt/keyrings/docker.asc
  - >-
    echo "deb [arch=$(dpkg --print-architecture)
    signed-by=/etc/apt/keyrings/docker.asc]
    https://download.docker.com/linux/ubuntu
    $(. /etc/os-release && echo "${VERSION_CODENAME}") stable"
    >/etc/apt/sources.list.d/docker.list
  - apt-get update
  - >-
    apt-get install -y containerd.io docker-ce docker-ce-cli
    docker-buildx-plugin docker-compose-plugin
  - install -d -o {{.User}} -g {{.User}} /opt/interview
{{- if ne .User "root"}}
  - usermod -aG docker {{.User}}
{{- end}}
`

var userDataParsed = template.Must(template.New("userdata").Parse(userDataTmpl))

// UserData renders the cloud-init document for a VM whose ssh login is user.
func UserData(user string) (string, error) {
	var b strings.Builder
	if err := userDataParsed.Execute(&b, struct{ User string }{User: user}); err != nil {
		return "", err
	}
	return b.String(), nil
}
