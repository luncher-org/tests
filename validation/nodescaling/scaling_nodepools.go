package nodescaling

import (
	"testing"

	"github.com/rancher/shepherd/clients/ec2"
	"github.com/rancher/shepherd/clients/rancher"
	"github.com/rancher/shepherd/extensions/clusters"
	"github.com/rancher/shepherd/extensions/clusters/aks"
	"github.com/rancher/shepherd/extensions/clusters/eks"
	"github.com/rancher/shepherd/extensions/clusters/gke"
	"github.com/rancher/shepherd/pkg/config"
	"github.com/rancher/tests/actions/machinepools"
	"github.com/rancher/tests/actions/provisioning"
	rke1 "github.com/rancher/tests/actions/rke1/nodepools"
	"github.com/rancher/tests/actions/workloads/pods"
	"github.com/stretchr/testify/require"
)

const (
	ProvisioningSteveResourceType = "provisioning.cattle.io.cluster"
	defaultNamespace              = "fleet-default"
)

// ScalingRKE2K3SNodePools scales the node pools of an RKE2 or K3S cluster.
func ScalingRKE2K3SNodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles machinepools.NodeRoles) {
	cluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(t, err)

	if nodeRoles.Windows {
		nodeRoles.Quantity++
	}

	clusterResp, err := machinepools.ScaleMachinePoolNodes(client, cluster, nodeRoles)
	require.NoError(t, err)

	pods.VerifyReadyDaemonsetPods(t, client, clusterResp)

	updatedCluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(t, err)

	if nodeRoles.Windows {
		nodeRoles.Quantity--
	} else {
		nodeRoles.Quantity = -nodeRoles.Quantity
	}

	scaledClusterResp, err := machinepools.ScaleMachinePoolNodes(client, updatedCluster, nodeRoles)
	require.NoError(t, err)

	pods.VerifyReadyDaemonsetPods(t, client, scaledClusterResp)
}

// ScalingRKE2K3SCustomClusterPools scales the node pools of an RKE2 or K3S custom cluster.
func ScalingRKE2K3SCustomClusterPools(t *testing.T, client *rancher.Client, clusterID string, nodeProvider string, nodeRoles machinepools.NodeRoles, awsEC2Configs *ec2.AWSEC2Configs) {
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}

	for _, nodeRoles := range []machinepools.NodeRoles{nodeRoles} {
		var finalRoleCommand string

		if nodeRoles.ControlPlane {
			finalRoleCommand += " --controlplane"
		}

		if nodeRoles.Etcd {
			finalRoleCommand += " --etcd"
		}

		if nodeRoles.Worker {
			finalRoleCommand += " --worker"
		}

		if nodeRoles.Windows {
			finalRoleCommand += " --windows"
		}

		quantityPerPool = append(quantityPerPool, nodeRoles.Quantity)
		rolesPerPool = append(rolesPerPool, finalRoleCommand)

		for i := int32(0); i < nodeRoles.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}

	var externalNodeProvider provisioning.ExternalNodeProvider
	externalNodeProvider = provisioning.ExternalNodeProviderSetup(nodeProvider)

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool, awsEC2Configs)
	require.NoError(t, err)

	cluster, err := client.Steve.SteveType(ProvisioningSteveResourceType).ByID(clusterID)
	require.NoError(t, err)

	err = provisioning.AddRKE2K3SCustomClusterNodes(client, cluster, nodes, rolesPerNode)
	require.NoError(t, err)

	pods.VerifyReadyDaemonsetPods(t, client, cluster)
	require.NoError(t, err)

	clusterID, err = clusters.GetClusterIDByName(client, cluster.Name)
	require.NoError(t, err)

	err = provisioning.DeleteRKE2K3SCustomClusterNodes(client, clusterID, cluster, nodes)
	require.NoError(t, err)

	err = externalNodeProvider.NodeDeletionFunc(client, nodes)
	require.NoError(t, err)
}

// ScalingRKE1NodePools scales the node pools of an RKE1 cluster.
func ScalingRKE1NodePools(t *testing.T, client *rancher.Client, clusterID string, nodeRoles rke1.NodeRoles) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	node, err := rke1.MatchRKE1NodeRoles(client, cluster, nodeRoles)
	require.NoError(t, err)

	_, err = rke1.ScaleNodePoolNodes(client, cluster, node, nodeRoles)
	require.NoError(t, err)

	nodeRoles.Quantity = -nodeRoles.Quantity
	_, err = rke1.ScaleNodePoolNodes(client, cluster, node, nodeRoles)
	require.NoError(t, err)
}

// ScalingRKE1CustomClusterPools scales the node pools of an RKE1 custom cluster.
func ScalingRKE1CustomClusterPools(t *testing.T, client *rancher.Client, clusterID string, nodeProvider string, nodeRoles rke1.NodeRoles) {
	rolesPerNode := []string{}
	quantityPerPool := []int32{}
	rolesPerPool := []string{}

	for _, pool := range []rke1.NodeRoles{nodeRoles} {
		var finalRoleCommand string

		if pool.ControlPlane {
			finalRoleCommand += " --controlplane"
		}

		if pool.Etcd {
			finalRoleCommand += " --etcd"
		}

		if pool.Worker {
			finalRoleCommand += " --worker"
		}

		quantityPerPool = append(quantityPerPool, int32(pool.Quantity))
		rolesPerPool = append(rolesPerPool, finalRoleCommand)

		for i := int64(0); i < pool.Quantity; i++ {
			rolesPerNode = append(rolesPerNode, finalRoleCommand)
		}
	}

	var externalNodeProvider provisioning.ExternalNodeProvider
	externalNodeProvider = provisioning.ExternalNodeProviderSetup(nodeProvider)

	awsEC2Configs := new(ec2.AWSEC2Configs)
	config.LoadConfig(ec2.ConfigurationFileKey, awsEC2Configs)

	nodes, err := externalNodeProvider.NodeCreationFunc(client, rolesPerPool, quantityPerPool, awsEC2Configs)
	require.NoError(t, err)

	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	err = provisioning.AddRKE1CustomClusterNodes(client, cluster, nodes, rolesPerNode)
	require.NoError(t, err)

	err = provisioning.DeleteRKE1CustomClusterNodes(client, cluster, nodes)
	require.NoError(t, err)

	err = externalNodeProvider.NodeDeletionFunc(client, nodes)
	require.NoError(t, err)
}

// ScalingAKSNodePools scales the node pools of an AKS cluster.
func ScalingAKSNodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *aks.NodePool) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := aks.ScalingAKSNodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.NodeCount = -*nodePool.NodeCount

	_, err = aks.ScalingAKSNodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}

// ScalingEKSNodePools scales the node pools of an EKS cluster.
func ScalingEKSNodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *eks.NodeGroupConfig) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := eks.ScalingEKSNodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.DesiredSize = -*nodePool.DesiredSize

	_, err = eks.ScalingEKSNodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}

// ScalingGKENodePools scales the node pools of a GKE cluster.
func ScalingGKENodePools(t *testing.T, client *rancher.Client, clusterID string, nodePool *gke.NodePool) {
	cluster, err := client.Management.Cluster.ByID(clusterID)
	require.NoError(t, err)

	clusterResp, err := gke.ScalingGKENodePoolsNodes(client, cluster, nodePool)
	require.NoError(t, err)

	*nodePool.InitialNodeCount = -*nodePool.InitialNodeCount

	_, err = gke.ScalingGKENodePoolsNodes(client, clusterResp, nodePool)
	require.NoError(t, err)
}
