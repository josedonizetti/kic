package oci

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/medyagh/kic/pkg/command"
	"github.com/medyagh/kic/pkg/config/cri"
	"github.com/pkg/errors"
	"sigs.k8s.io/kind/pkg/exec"
)

// can be podman
const DefaultOCI = "docker"

// Inspect return low-level information on containers
func Inspect(localRunner command.Runner, containerNameOrID, format string) ([]string, error) {
	args := []string{
		"inspect", "-f", format, containerNameOrID,
	}

	var b bytes.Buffer
	err := localRunner.CombinedOutputTo(strings.Join(args, " "), &b)
	lines := []string{}
	scanner := bufio.NewScanner(&b)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err != nil {
		return lines, errors.Wrapf(err, "Inspect container %s output: %v", containerNameOrID, lines)
	}

	return lines, nil
}

// NetworkInspect displays detailed information on one or more networks
func NetworkInspect(networkNames []string, format string) ([]string, error) {
	cmd := exec.Command("docker", "network", "inspect",
		"-f", format,
		strings.Join(networkNames, " "),
	)
	return exec.CombinedOutputLines(cmd)
}

/*
This is adapated from:
https://github.com/kubernetes/kubernetes/blob/07a5488b2a8f67add543da72e8819407d8314204/pkg/kubelet/dockershim/helpers.go#L115-L155
*/
// generateMountBindings converts the mount list to a list of strings that
// can be understood by docker
// '<HostPath>:<ContainerPath>[:options]', where 'options'
// is a comma-separated list of the following strings:
// 'ro', if the path is read only
// 'Z', if the volume requires SELinux relabeling
func generateMountBindings(mounts ...cri.Mount) []string {
	result := make([]string, 0, len(mounts))
	for _, m := range mounts {
		bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
		var attrs []string
		if m.Readonly {
			attrs = append(attrs, "ro")
		}
		// Only request relabeling if the pod provides an SELinux context. If the pod
		// does not provide an SELinux context relabeling will label the volume with
		// the container's randomly allocated MCS label. This would restrict access
		// to the volume to the container which mounts it first.
		if m.SelinuxRelabel {
			attrs = append(attrs, "Z")
		}
		switch m.Propagation {
		case cri.MountPropagationNone:
			// noop, private is default
		case cri.MountPropagationBidirectional:
			attrs = append(attrs, "rshared")
		case cri.MountPropagationHostToContainer:
			attrs = append(attrs, "rslave")
		default:
			// Falls back to "private"
		}

		if len(attrs) > 0 {
			bind = fmt.Sprintf("%s:%s", bind, strings.Join(attrs, ","))
		}
		// our specific modification is the following line: make this a docker flag
		bind = fmt.Sprintf("--volume=%s", bind)
		result = append(result, bind)
	}
	return result
}

// PullIfNotPresent pulls docker image if not present back off exponentially
func PullIfNotPresent(image string, forceUpdate bool, maxWait time.Duration) error {
	cmd := exec.Command(DefaultOCI, "inspect", "--type=image", image)
	err := cmd.Run()
	if err == nil && forceUpdate == false {
		return nil // if presents locally and not force
	}
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxWait
	f := func() error {
		return pull(image)
	}
	return backoff.Retry(f, b)
}

// Pull pulls an image, retrying up to retries times
func pull(image string) error {
	err := exec.Command(DefaultOCI, "pull", image).Run()
	if err != nil {
		return fmt.Errorf("error pull image %s : %v", image, err)
	}
	return err
}

// UsernsRemap checks if userns-remap is enabled in dockerd
func UsernsRemap() bool {
	cmd := exec.Command(DefaultOCI, "info", "--format", "'{{json .SecurityOptions}}'")
	lines, err := exec.CombinedOutputLines(cmd)
	if err != nil {
		return false
	}
	if len(lines) > 0 {
		if strings.Contains(lines[0], "name=userns") {
			return true
		}
	}
	return false
}

func generatePortMappings(portMappings ...cri.PortMapping) []string {
	result := make([]string, 0, len(portMappings))
	for _, pm := range portMappings {
		var hostPortBinding string
		if pm.ListenAddress != "" {
			hostPortBinding = net.JoinHostPort(pm.ListenAddress, fmt.Sprintf("%d", pm.HostPort))
		} else {
			hostPortBinding = fmt.Sprintf("%d", pm.HostPort)
		}
		publish := fmt.Sprintf("--publish=%s:%d", hostPortBinding, pm.ContainerPort)
		result = append(result, publish)
	}
	return result
}
