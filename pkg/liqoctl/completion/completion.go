// Copyright 2019-2023 The Liqo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package completion

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	discoveryv1alpha1 "github.com/liqotech/liqo/apis/discovery/v1alpha1"
	networkingv1alpha1 "github.com/liqotech/liqo/apis/networking/v1alpha1"
	offloadingv1alpha1 "github.com/liqotech/liqo/apis/offloading/v1alpha1"
	virtualkubeletv1alpha1 "github.com/liqotech/liqo/apis/virtualkubelet/v1alpha1"
	identitymanager "github.com/liqotech/liqo/pkg/identityManager"
	"github.com/liqotech/liqo/pkg/liqoctl/factory"
	"github.com/liqotech/liqo/pkg/utils/slice"
	utilsvirtualnode "github.com/liqotech/liqo/pkg/utils/virtualnode"
)

// NoLimit is a constant to specify that autocompletion is not limited depending on the number of arguments.
const NoLimit = -1

// FnType represents the type of a cobra autocompletion function.
type FnType func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

type retriever func(ctx context.Context, f *factory.Factory) ([]string, error)

func common(ctx context.Context, f *factory.Factory, argsLimit int, retrieve retriever) FnType {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if argsLimit != NoLimit && len(args) >= argsLimit {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		if err := f.Initialize(); err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		values, err := retrieve(ctx, f)
		if err != nil {
			return nil, cobra.ShellCompDirectiveError
		}

		var output []string
		for _, value := range values {
			if strings.HasPrefix(value, toComplete) && !slice.ContainsString(args, value) {
				output = append(output, value)
			}
		}

		return output, cobra.ShellCompDirectiveNoFileComp
	}
}

// Enumeration returns a function to autocomplete enumeration values.
func Enumeration(values []string) FnType {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return values, cobra.ShellCompDirectiveNoFileComp
	}
}

// Namespaces returns a function to autocomplete namespace names.
func Namespaces(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var namespaces corev1.NamespaceList
		if err := f.CRClient.List(ctx, &namespaces); err != nil {
			return nil, err
		}

		var names []string
		for i := range namespaces.Items {
			names = append(names, namespaces.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// OffloadedNamespaces returns a function to autocomplete namespace names (only offloaded ones).
func OffloadedNamespaces(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var nsoff offloadingv1alpha1.NamespaceOffloadingList
		if err := f.CRClient.List(ctx, &nsoff); err != nil {
			return nil, err
		}

		var names []string
		for i := range nsoff.Items {
			names = append(names, nsoff.Items[i].Namespace)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// Nodes returns a function to autocomplete node names.
func Nodes(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var nodes corev1.NodeList
		if err := f.CRClient.List(ctx, &nodes); err != nil {
			return nil, err
		}

		var names []string
		for i := range nodes.Items {
			names = append(names, nodes.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// VirtualNodes returns a function to autocomplete virtual node names.
func VirtualNodes(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var virtualNodes virtualkubeletv1alpha1.VirtualNodeList
		if err := f.CRClient.List(ctx, &virtualNodes, client.InNamespace(f.Namespace)); err != nil {
			return nil, err
		}

		var names []string
		for i := range virtualNodes.Items {
			names = append(names, virtualNodes.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// LabelsSelector returns a function to autocomplete selector labels.
func LabelsSelector(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		// labelsCounter contains a 'key=value' string as key and the number of times it appears as value.
		labelsCounter := map[string]int{}
		var virtualNodes virtualkubeletv1alpha1.VirtualNodeList
		if err := f.CRClient.List(ctx, &virtualNodes); err != nil {
			return nil, err
		}
		for i := range virtualNodes.Items {
			labelSet, err := utilsvirtualnode.GetLabelSelectors(ctx, f.CRClient, &virtualNodes.Items[i])
			if err != nil {
				return nil, err
			}
			for k, v := range labelSet {
				addLabelSelector(k, v, labelsCounter)
			}
		}
		return parseLabelSelectors(labelsCounter, len(virtualNodes.Items)), nil
	}
	return common(ctx, f, argsLimit, retriever)
}

func addLabelSelector(key, value string, labelset map[string]int) {
	entry := fmt.Sprintf("%s=%s", key, value)
	if _, ok := labelset[entry]; ok {
		labelset[entry]++
		return
	}
	labelset[entry] = 1
}

func parseLabelSelectors(labelset map[string]int, max int) []string {
	var output []string
	for k, v := range labelset {
		if v != max {
			// this means that the label is not present in all virtualnodes or node, so can be used as selector
			output = append(output, k)
		}
	}
	return output
}

// ForeignClusters returns a function to autocomplete ForeignCluster names.
func ForeignClusters(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var foreignClusters discoveryv1alpha1.ForeignClusterList
		if err := f.CRClient.List(ctx, &foreignClusters); err != nil {
			return nil, err
		}

		var names []string
		for i := range foreignClusters.Items {
			names = append(names, foreignClusters.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// ClusterIDs returns a function to autocomplete ForeignCluster cluster IDs.
func ClusterIDs(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var foreignClusters discoveryv1alpha1.ForeignClusterList
		if err := f.CRClient.List(ctx, &foreignClusters); err != nil {
			return nil, err
		}

		var ids []string
		for i := range foreignClusters.Items {
			ids = append(ids, foreignClusters.Items[i].Spec.ClusterIdentity.ClusterID)
		}
		return ids, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// ClusterNames returns a function to autocomplete ForeignCluster cluster names.
func ClusterNames(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var foreignClusters discoveryv1alpha1.ForeignClusterList
		if err := f.CRClient.List(ctx, &foreignClusters); err != nil {
			return nil, err
		}

		var names []string
		for i := range foreignClusters.Items {
			names = append(names, foreignClusters.Items[i].Spec.ClusterIdentity.ClusterName)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// KubeconfigSecretNames returns a function to autocomplete kubeconfig secret names.
func KubeconfigSecretNames(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		matchingLabels := client.MatchingLabels{
			identitymanager.CertificateAvailableLabel: "true",
		}

		var secrets corev1.SecretList
		if err := f.CRClient.List(ctx, &secrets, client.InNamespace(f.Namespace), matchingLabels); err != nil {
			return nil, err
		}

		var names []string
		for i := range secrets.Items {
			names = append(names, secrets.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// PVCs returns a function to autocomplete PVC names.
func PVCs(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var pvcs corev1.PersistentVolumeClaimList
		if err := f.CRClient.List(ctx, &pvcs, client.InNamespace(f.Namespace)); err != nil {
			return nil, err
		}

		var names []string
		for i := range pvcs.Items {
			names = append(names, pvcs.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}

// Gateway returns a function to autocomplete Gateway (server or client) names.
func Gateway(ctx context.Context, f *factory.Factory, argsLimit int) FnType {
	retriever := func(ctx context.Context, f *factory.Factory) ([]string, error) {
		var names []string

		var gwServers networkingv1alpha1.GatewayServerList
		if err := f.CRClient.List(ctx, &gwServers, client.InNamespace(f.Namespace)); err != nil {
			return nil, err
		}
		for i := range gwServers.Items {
			names = append(names, gwServers.Items[i].Name)
		}

		var gwClients networkingv1alpha1.GatewayClientList
		if err := f.CRClient.List(ctx, &gwClients, client.InNamespace(f.Namespace)); err != nil {
			return nil, err
		}
		for i := range gwClients.Items {
			names = append(names, gwClients.Items[i].Name)
		}
		return names, nil
	}

	return common(ctx, f, argsLimit, retriever)
}
