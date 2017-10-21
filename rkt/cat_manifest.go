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

package main

import (
	"encoding/json"

	"github.com/rkt/rkt/pkg/pod"
	"github.com/rkt/rkt/rkt/flag"
	"github.com/spf13/cobra"
)

type catManifestOptions struct {
	podFilename string
	prettyPrint bool

	pods []string
}

func NewCatManifestCommand() *cobra.Command {
	var (
		opts catManifestOptions
		err  error
	)

	cmdCatManifest := cobra.Command{
		Use:   "cat-manifest --uuid-file=FILE | UUID ...",
		Short: "Inspect and print the pod manifest",
		Long:  `UUID should be the UUID of a pod`,
		Run: runWrapper(func(cmd *cobra.Command, args []string) int {
			opts.pods, err = flag.Pods(opts.podFilename, args)
			if err != nil {
				stderr.Print(err.Error())
				cmd.Usage()
				return 254
			}
			return runCatManifest(cmd, &opts)
		}),
	}

	flags := cmdCatManifest.Flags()
	flags.BoolVar(&opts.prettyPrint, "pretty-print", true, "apply indent to format the output")
	flags.StringVar(&opts.podFilename, "uuid-file", "", "read pod UUID from file instead of argument")

	return &cmdCatManifest
}

func init() {
	cmdRkt.AddCommand(NewCatManifestCommand())
}

func runCatManifest(cmd *cobra.Command, opts *catManifestOptions) int {
	if len(opts.pods) > 1 {
		stderr.Printf("maximum one UUID is expected, got %d", len(opts.pods))
		return 254
	}

	pod, err := pod.PodFromUUIDString(getDataDir(), opts.pods[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 254
	}

	defer pod.Close()

	_, manifest, err := pod.PodManifest()
	if err != nil {
		return 254
	}

	var b []byte
	if opts.prettyPrint {
		b, err = json.MarshalIndent(manifest, "", "\t")
	} else {
		b, err = json.Marshal(manifest)
	}
	if err != nil {
		stderr.PrintE("cannot read the pod manifest", err)
		return 254
	}

	stdout.Print(string(b))
	return 0
}
