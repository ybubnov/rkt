package flag

import (
	"fmt"

	"github.com/hashicorp/errwrap"

	"github.com/rkt/rkt/pkg/pod"
)

// Pods reads the POD identifier from the pod file when not empty or
// returns the given pods. When neither POD file nor the list of pods
// is specified, an error is returned.
func Pods(podFilename string, pods []string) ([]string, error) {
	if podFilename != "" && len(pods) == 0 {
		podUUID, err := pod.ReadUUIDFromFile(podFilename)
		if err != nil {
			innerErr := fmt.Errorf("unable to resolve UUID from file %q", podFilename)
			return nil, errwrap.Wrap(innerErr, err)
		}
		return []string{podUUID}, nil
	}

	if podFilename == "" && len(pods) > 0 {
		return pods, nil
	}

	return nil, fmt.Errorf("either UUID file or a list of UUIDs is required")
}
