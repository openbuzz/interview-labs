package stack

import (
	"strings"
	"testing"
)

func TestUserDataInstallsDockerAndPayloadDir(t *testing.T) {
	ud, err := UserData("ubuntu")
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"#cloud-config",
		"docker-ce",
		"docker-buildx-plugin",
		"docker-compose-plugin",
		"install -d -o ubuntu -g ubuntu /opt/interview",
		"usermod -aG docker ubuntu",
	} {
		if !strings.Contains(ud, want) {
			t.Errorf("user data missing %q:\n%s", want, ud)
		}
	}
}

func TestUserDataRootSkipsGroupAdd(t *testing.T) {
	ud, err := UserData("root")
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(ud, "usermod") {
		t.Errorf("root user data must not usermod:\n%s", ud)
	}
	if !strings.Contains(ud, "install -d -o root -g root /opt/interview") {
		t.Errorf("payload dir line missing:\n%s", ud)
	}
}
