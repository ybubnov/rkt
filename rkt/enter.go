// Copyright 2014 The rkt Authors
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
	"errors"
	"fmt"

	"github.com/appc/spec/schema/types"
	"github.com/hashicorp/errwrap"
	"github.com/rkt/rkt/pkg/pod"
	"github.com/rkt/rkt/rkt/flag"
	"github.com/rkt/rkt/stage0"
	"github.com/rkt/rkt/store/imagestore"
	"github.com/rkt/rkt/store/treestore"
	"github.com/spf13/cobra"
)

const (
	defaultCmd = "/bin/bash"
)

type enterOptions struct {
	appName     string
	podFilename string

	pods []string
	args []string
}

func NewEnterCommand() *cobra.Command {
	var (
		opts enterOptions
		err  error
	)

	cmdEnter := cobra.Command{
		Use:   "enter [--app=APPNAME] --uuid-file=FILE | UUID [CMD [ARGS ...]]",
		Short: "Enter the namespaces of an app within a rkt pod",

		Long: `UUID should be the UUID of a running pod.

By default the CMD that is run is /bin/bash, providing the user with shell
access to the running pod.`,
		Run: ensureSuperuser(runWrapper(func(cmd *cobra.Command, args []string) int {
			opts.pods, err = flag.Pods(opts.podFilename, args)
			if err != nil {
				stderr.Print(err.Error())
				cmd.Usage()
				return 254
			}

			// When the POD filename is empty, the first argument is
			// treated as a POD UUID, and the rest as command to execute.
			if opts.podFilename == "" {
				opts.pods, opts.args = opts.pods[:0], opts.pods[1:]
			}
			return runEnter(cmd, &opts)
		})),
	}

	flags := cmdEnter.Flags()
	flags.StringVar(&opts.appName, "app", "", "name of the app to enter within the specified pod")
	flags.StringVar(&opts.podFilename, "uuid-file", "", "read pod UUID from file instead of argument")

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. This is need to permit to correctly handle
	// multiple "IMAGE -- imageargs ---"  options
	flags.SetInterspersed(false)

	return &cmdEnter
}

func init() {
	cmdRkt.AddCommand(NewEnterCommand())
}

func runEnter(cmd *cobra.Command, opts *enterOptions) int {
	p, err := pod.PodFromUUIDString(getDataDir(), opts.pods[0])
	if err != nil {
		stderr.PrintE("problem retrieving pod", err)
		return 254
	}
	defer p.Close()

	if p.State() != pod.Running {
		stderr.Printf("pod %q is not currently running", p.UUID)
		return 254
	}

	podPID, err := p.ContainerPid1()
	if err != nil {
		stderr.PrintE(fmt.Sprintf("unable to determine the pid for pod %q", p.UUID), err)
		return 254
	}

	appName, err := getAppName(p, opts.appName)
	if err != nil {
		stderr.PrintE("unable to determine app name", err)
		return 254
	}

	argv, err := getEnterArgv(p, opts.args)
	if err != nil {
		stderr.PrintE("enter failed", err)
		return 254
	}

	s, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 254
	}

	ts, err := treestore.NewStore(treeStoreDir(), s)
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 254
	}

	stage1TreeStoreID, err := p.GetStage1TreeStoreID()
	if err != nil {
		stderr.PrintE("error getting stage1 treeStoreID", err)
		return 254
	}

	stage1RootFS := ts.GetRootFS(stage1TreeStoreID)

	if err = stage0.Enter(p.Path(), podPID, *appName, stage1RootFS, argv); err != nil {
		stderr.PrintE("enter failed", err)
		return 254
	}
	// not reached when stage0.Enter execs /enter
	return 0
}

// getAppName returns the app name to enter
// If one was supplied in the flags then it's simply returned
// If the PM contains a single app, that app's name is returned
// If the PM has multiple apps, the names are printed and an error is returned
func getAppName(p *pod.Pod, appName string) (*types.ACName, error) {
	if appName != "" {
		return types.NewACName(appName)
	}

	// figure out the app name, or show a list if multiple are present
	_, m, err := p.PodManifest()
	if err != nil {
		return nil, errwrap.Wrap(errors.New("error reading pod manifest"), err)
	}

	switch len(m.Apps) {
	case 0:
		return nil, fmt.Errorf("pod contains zero apps")
	case 1:
		return &m.Apps[0].Name, nil
	default:
	}

	stderr.Print("pod contains multiple apps:")
	for _, ra := range m.Apps {
		stderr.Printf("\t%v", ra.Name)
	}

	return nil, fmt.Errorf("specify app using \"rkt enter --app= ...\"")
}

// getEnterArgv returns the argv to use for entering the pod
func getEnterArgv(p *pod.Pod, cmdArgs []string) ([]string, error) {
	var argv []string
	if len(cmdArgs) < 2 {
		stderr.Printf("no command specified, assuming %q", defaultCmd)
		argv = []string{defaultCmd}
	} else {
		argv = cmdArgs[1:]
	}

	return argv, nil
}
