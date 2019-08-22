package node

import (
	"fmt"
	"strings"

	"github.com/medyagh/kic/pkg/exec"
	"github.com/medyagh/kic/pkg/node/cri"
	"github.com/pkg/errors"
)

// Spec describes a node to create purely from the container aspect
// this does not inlude eg starting kubernetes (see actions for that)
type Spec struct {
	Name              string
	Profile           string
	Role              string
	Image             string
	ExtraMounts       []cri.Mount
	ExtraPortMappings []cri.PortMapping
	APIServerPort     int32
	APIServerAddress  string
	IPv6              bool
}

func (d *Spec) Create(cmder exec.Cmder) (node *Node, err error) {
	switch d.Role {
	case "control-plane":
		node, err := CreateControlPlaneNode(d.Name, d.Image, ClusterLabelKey+d.Profile, d.APIServerAddress, d.APIServerPort, d.ExtraMounts, d.ExtraPortMappings, cmder)
		return node, err
	default:
		return nil, fmt.Errorf("unknown node role: %s", d.Role)
	}
	return node, err
}

func (d *Spec) Stop() error {
	cmd := exec.Command("docker", "pause", d.Name)
	_, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		return errors.Wrapf(err, "stopping node")
	}
	return nil
}

func (d *Spec) Delete() error {
	cmd := exec.Command("docker", "rm", "-f", "-v", d.Name)
	_, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		return errors.Wrapf(err, "deleting node")
	}
	return nil
}

// ListNodes lists all the nodes (containers) created by kic on the system
func (d *Spec) ListNodes() ([]string, error) {
	args := []string{
		"ps",
		"-q",         // quiet output for parsing
		"-a",         // show stopped nodes
		"--no-trunc", // don't truncate
		// filter for nodes with the cluster label
		"--filter", "label=" + ClusterLabelKey + d.Profile,
		// format to include friendly name and the cluster name
		"--format", fmt.Sprintf(`{{.Names}}\t{{.Label "%s"}}`, ClusterLabelKey+d.Profile),
	}
	cmd := exec.Command("docker", args...)
	lines, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to list containers for %s", d.Profile))

	}
	names := []string{}
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid output when listing containers: %s", line)

		}
		ns := strings.Split(parts[0], ",")
		names = append(names, ns...)
	}
	return names, nil

}
