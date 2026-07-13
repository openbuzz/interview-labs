package stack

import "strings"

// Override renders the engine-generated compose override: workspace/lab
// mounts when a bundle is staged, kind network + kubeconfig when the bundle
// ships a cluster. This file is the only compose delta packs get — bundle
// compose fragments are deliberately not a thing (spec §9).
func Override(workspace, lab, kind bool) string {
	var vols, nets []string
	if workspace {
		vols = append(vols, "      - ./payload/workspace:/home/user/scenarios:rw")
	}
	if lab {
		vols = append(vols, "      - ./payload/lab:/opt/interview/lab:ro")
	}
	if kind {
		vols = append(vols, "      - ./payload/kubeconfig:/home/user/.kube/config:ro")
		nets = append(nets, "    networks: [default, kind]")
	}

	var b strings.Builder
	b.WriteString("services:\n  vscode:\n")
	for _, n := range nets {
		b.WriteString(n + "\n")
	}
	b.WriteString("    volumes:\n")
	for _, v := range vols {
		b.WriteString(v + "\n")
	}
	if kind {
		b.WriteString("networks:\n  kind:\n    external: true\n")
	}
	return b.String()
}
