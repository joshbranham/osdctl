package dynatrace

import (
	"fmt"
	"strings"

	ocmutils "github.com/openshift/osdctl/pkg/utils"
)

type HCPCluster struct {
	name                  string
	internalID            string
	externalID            string
	managementClusterID   string
	klusterletNS          string
	hostedNS              string
	hcpNamespace          string
	managementClusterName string
	DynatraceURL          string
	serviceClusterID      string
	serviceClusterName    string
}

var ErrUnsupportedCluster = fmt.Errorf("not an HCP or MC Cluster")

func FetchClusterDetails(clusterKey string) (hcpCluster HCPCluster, error error) {
	hcpCluster = HCPCluster{}
	if err := ocmutils.IsValidClusterKey(clusterKey); err != nil {
		return hcpCluster, err
	}
	connection, err := ocmutils.CreateConnection()
	if err != nil {
		return HCPCluster{}, err
	}
	defer connection.Close()

	cluster, err := ocmutils.GetCluster(connection, clusterKey)
	if err != nil {
		return HCPCluster{}, err
	}

	if !cluster.Hypershift().Enabled() {
		isMC, err := ocmutils.IsManagementCluster(cluster.ID())
		if !isMC || err != nil {
			// if the cluster is not a HCP or MC, then return an error
			return HCPCluster{}, ErrUnsupportedCluster
		} else {
			// if the cluster is not a HCP but a MC, then return a just relevant info for HCPCluster Object
			hcpCluster.managementClusterID = cluster.ID()
			hcpCluster.managementClusterName = cluster.Name()
			url, err := ocmutils.GetDynatraceURLFromLabel(hcpCluster.managementClusterID)
			if err != nil {
				return HCPCluster{}, fmt.Errorf("the Dynatrace Environment URL could not be determined. \nPlease refer the SOP to determine the correct Dynatrace Tenant URL- https://github.com/openshift/ops-sop/tree/master/dynatrace#what-environments-are-there \n\nError Details - %s", err)
			}
			hcpCluster.DynatraceURL = url
			return hcpCluster, nil
		}
	}

	mgmtCluster, err := ocmutils.GetManagementCluster(cluster.ID())
	if err != nil {
		return HCPCluster{}, fmt.Errorf("error retreiving Management Cluster for given HCP %s", err)
	}
	svcCluster, err := ocmutils.GetServiceCluster(cluster.ID())
	if err != nil {
		return HCPCluster{}, fmt.Errorf("error retreiving Service Cluster for given HCP %s", err)
	}
	hcpCluster.hcpNamespace, err = ocmutils.GetHCPNamespace(cluster.ID())
	if err != nil {
		return HCPCluster{}, fmt.Errorf("error retreiving HCP Namespace for given cluster")
	}
	hcpCluster.klusterletNS = fmt.Sprintf("klusterlet-%s", cluster.ID())
	hcpCluster.hostedNS = strings.SplitAfter(hcpCluster.hcpNamespace, cluster.ID())[0]

	url, err := ocmutils.GetDynatraceURLFromLabel(mgmtCluster.ID())
	if err != nil {
		return HCPCluster{}, fmt.Errorf("the Dynatrace Environemnt URL could not be determined. \nPlease refer the SOP to determine the correct Dyntrace Tenant URL- https://github.com/openshift/ops-sop/tree/master/dynatrace#what-environments-are-there \n\nError Details - %s", err)
	}

	hcpCluster.DynatraceURL = url
	hcpCluster.internalID = cluster.ID()
	hcpCluster.externalID = cluster.ExternalID()
	hcpCluster.managementClusterID = mgmtCluster.ID()
	hcpCluster.name = cluster.Name()
	hcpCluster.managementClusterName = mgmtCluster.Name()
	hcpCluster.serviceClusterID = svcCluster.ID()
	hcpCluster.serviceClusterName = svcCluster.Name()

	return hcpCluster, nil
}
