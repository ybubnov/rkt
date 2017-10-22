// Copyright 2015 The rkt Authors
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
	"os"

	"github.com/rkt/rkt/pkg/pod"
	"github.com/rkt/rkt/rkt/flag"
	"github.com/spf13/cobra"
)

type rmOptions struct {
	podFilename string

	pods []string
}

func NewRmCommand() *cobra.Command {
	var (
		opts rmOptions
		err  error
	)

	cmdRm := cobra.Command{
		Use:   "rm --uuid-file=FILE | UUID ...",
		Short: "Remove all files and resources associated with an exited pod",
		Long:  `Unlike gc, rm allows users to remove specific pods.`,
		Run: ensureSuperuser(runWrapper(func(cmd *cobra.Command, args []string) int {
			opts.pods, err = flag.Pods(opts.podFilename, args)
			if err != nil {
				stderr.Print(err.Error())
				return 254
			}
			return runRm(cmd, &opts)
		})),
	}

	flags := cmdRm.Flags()
	flags.StringVar(&opts.podFilename, "uuid-file", "", "read pod UUID from file instead of argument")

	return &cmdRm
}

func init() {
	cmdRkt.AddCommand(NewRmCommand())
}

func runRm(cmd *cobra.Command, opts *rmOptions) (exit int) {
	var ret int

	for _, podUUID := range opts.pods {
		p, err := pod.PodFromUUIDString(getDataDir(), podUUID)
		if err != nil {
			ret = 254
			stderr.PrintE("cannot get pod", err)
			continue
		}
		defer p.Close()

		if removePod(p) {
			stdout.Printf("%q", p.UUID)
		} else {
			ret = 254
		}
	}

	if ret == 254 {
		stderr.Print("failed to remove one or more pods")
	}

	return ret
}

func removePod(p *pod.Pod) bool {
	switch p.State() {
	case pod.Running:
		stderr.Printf("pod %q is currently running", p.UUID)
		return false

	case pod.Embryo, pod.Preparing:
		stderr.Printf("pod %q is currently being prepared", p.UUID)
		return false

	case pod.Deleting:
		stderr.Printf("pod %q is currently being deleted", p.UUID)
		return false

	case pod.AbortedPrepare:
		stderr.Printf("moving failed prepare %q to garbage", p.UUID)
		if err := p.ToGarbage(); err != nil && err != os.ErrNotExist {
			stderr.PrintE("rename error", err)
			return false
		}

	case pod.Prepared:
		stderr.Printf("moving expired prepared pod %q to garbage", p.UUID)
		if err := p.ToGarbage(); err != nil && err != os.ErrNotExist {
			stderr.PrintE("rename error", err)
			return false
		}

	// p.isExitedGarbage and p.isExited can be true at the same time. Test
	// the most specific case first.
	case pod.ExitedGarbage, pod.Garbage:

	case pod.Exited:
		if err := p.ToExitedGarbage(); err != nil && err != os.ErrNotExist {
			stderr.PrintE("rename error", err)
			return false
		}
	}

	if err := p.ExclusiveLock(); err != nil {
		stderr.PrintE("unable to acquire exclusive lock", err)
		return false
	}

	return deletePod(p)
}
