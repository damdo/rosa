/*
Copyright (c) 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubeletconfig

import (
	"fmt"
	"os"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/interactive"
	"github.com/openshift/rosa/pkg/interactive/confirm"
	. "github.com/openshift/rosa/pkg/kubeletconfig"
	"github.com/openshift/rosa/pkg/ocm"
	"github.com/openshift/rosa/pkg/rosa"
)

var Cmd = &cobra.Command{
	Use:     "kubeletconfig",
	Aliases: []string{"kubelet-config"},
	Short:   "Create a custom kubeletconfig for a cluster",
	Long:    "Create a custom kubeletconfig for a cluster",
	Example: `  # Create a custom kubeletconfig with a pod-pids-limit of 5000
  rosa create kubeletconfig --cluster=mycluster --pod-pids-limit=5000
  `,
	Run: run,
}

var args struct {
	podPidsLimit int
}

func init() {
	flags := Cmd.Flags()
	flags.SortFlags = false
	flags.IntVar(
		&args.podPidsLimit,
		PodPidsLimitOption,
		PodPidsLimitOptionDefaultValue,
		PodPidsLimitOptionUsage)

	ocm.AddClusterFlag(Cmd)
	interactive.AddFlag(flags)
}

func run(_ *cobra.Command, _ []string) {
	r := rosa.NewRuntime().WithOCM()
	defer r.Cleanup()

	clusterKey := r.GetClusterKey()
	cluster := r.FetchCluster()

	if cluster.Hypershift().Enabled() {
		r.Reporter.Errorf("Hosted Control Plane clusters do not support custom KubeletConfig configuration.")
		os.Exit(1)
	}

	if cluster.State() != cmv1.ClusterStateReady {
		r.Reporter.Errorf("Cluster '%s' is not yet ready. Current state is '%s'", clusterKey, cluster.State())
		os.Exit(1)
	}

	kubeletConfig, err := r.OCMClient.GetClusterKubeletConfig(cluster.ID())
	if err != nil {
		r.Reporter.Errorf("Failed getting KubeletConfig for cluster '%s': %s",
			cluster.ID(), err)
		os.Exit(1)
	}

	if kubeletConfig != nil {
		r.Reporter.Errorf("A custom KubeletConfig for cluster '%s' already exists. "+
			"You should edit it via 'rosa edit kubeletconfig'", clusterKey)
		os.Exit(1)
	}

	requestedPids, err := ValidateOrPromptForRequestedPidsLimit(args.podPidsLimit, clusterKey, nil, r)
	if err != nil {
		os.Exit(1)
	}

	prompt := fmt.Sprintf("Creating the custom KubeletConfig for cluster '%s' will cause all non-Control Plane "+
		"nodes to reboot. This may cause outages to your applications. Do you wish to continue?", clusterKey)

	if confirm.ConfirmRaw(prompt) {

		r.Reporter.Debugf("Creating KubeletConfig for cluster '%s'", clusterKey)
		kubeletConfigArgs := ocm.KubeletConfigArgs{PodPidsLimit: requestedPids}

		_, err = r.OCMClient.CreateKubeletConfig(cluster.ID(), kubeletConfigArgs)
		if err != nil {
			r.Reporter.Errorf("Failed creating custom KubeletConfig for cluster '%s': '%s'",
				clusterKey, err)
			os.Exit(1)
		}

		r.Reporter.Infof("Successfully created custom KubeletConfig for cluster '%s'", clusterKey)
		os.Exit(0)
	}

	r.Reporter.Infof("Creation of custom KubeletConfig for cluster '%s' aborted.", clusterKey)
}
