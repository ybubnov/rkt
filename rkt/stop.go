// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//+build linux

package main

import (
	"fmt"

	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"

	"github.com/rkt/rkt/pkg/pod"
	"github.com/rkt/rkt/rkt/flag"
	"github.com/rkt/rkt/stage0"
)

type stopOptions struct {
	force       bool
	podFilename string

	// A list of the POD UUIDs.
	pods []string
}

// NewStopCommand creates a new stop sub-command.
func NewStopCommand() *cobra.Command {
	var (
		opts stopOptions
		err  error
	)

	cmdStop := cobra.Command{
		Use:          "stop --uuid-file=FILE | UUID [UUIDS...]",
		Short:        "Stop a pod",
		SilenceUsage: true,
		Run: runWrapper(func(cmd *cobra.Command, args []string) int {
			opts.pods, err = flag.Pods(opts.podFilename, args)
			if err != nil {
				stderr.Print(err.Error())
				cmd.Usage()
				return 254
			}
			return runStop(cmd, &opts)
		}),
	}

	flags := cmdStop.Flags()
	flags.BoolVar(&opts.force, "force", false, "force stopping")
	flags.StringVar(&opts.podFilename, "uuid-file", "", "read pod UUID from file instead of argument")

	return &cmdStop
}

func init() {
	cmdRkt.AddCommand(NewStopCommand())
}

func runStop(cmd *cobra.Command, opts *stopOptions) int {
	var (
		errors  = 0
		dataDir = getDataDir()
	)

	for _, podUUID := range opts.pods {
		err := stopPod(dataDir, podUUID, opts.force)
		if err != nil {
			errors++
			stderr.PrintE(fmt.Sprintf("error stopping %q", podUUID), err)
			continue
		}

		stdout.Printf("%q", podUUID)
	}

	if errors > 0 {
		stderr.Error(fmt.Errorf("failed to stop %d pod(s)", errors))
		return 254
	}

	return 0
}

func stopPod(dataDir, uuid string, force bool) error {
	p, err := pod.PodFromUUIDString(dataDir, uuid)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("cannot get pod"), err)
	}

	defer p.Close()
	if p.IsAfterRun() {
		return fmt.Errorf("pod %q is already stopped", p.UUID)
	}

	if p.State() != pod.Running {
		return fmt.Errorf("pod %q is not running", p.UUID)
	}

	err = stage0.StopPod(p.Path(), force, p.UUID)
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("error stopping %q", p.UUID), err)
	}

	return nil
}
