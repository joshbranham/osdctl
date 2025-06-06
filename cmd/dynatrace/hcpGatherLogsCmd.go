package dynatrace

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/osdctl/cmd/common"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type GatherLogsOpts struct {
	Since     int
	Tail      int
	SortOrder string
	DestDir   string
	ClusterID string
}

func NewCmdHCPMustGather() *cobra.Command {
	g := &GatherLogsOpts{}

	hcpMgCmd := &cobra.Command{
		Use:     "gather-logs --cluster-id <cluster-identifier>",
		Aliases: []string{"gl"},
		Short:   "Gather all Pod logs and Application event from HCP",
		Long: `Gathers pods logs and evnets of a given HCP from Dynatrace.

  This command fetches the logs from the HCP namespace, the hypershift namespace and cert-manager related namespaces.
  Logs will be dumped to a directory with prefix hcp-must-gather.
		`,
		Example: `
  # Gather logs for a HCP cluster with cluster id hcp-cluster-id-123
  osdctl dt gather-logs --cluster-id hcp-cluster-id-123`,
		DisableAutoGenTag: true,
		Run: func(cmd *cobra.Command, args []string) {

			err := g.GatherLogs(g.ClusterID)
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	hcpMgCmd.Flags().IntVar(&g.Since, "since", 10, "Number of hours (integer) since which to pull logs and events")
	hcpMgCmd.Flags().IntVar(&g.Tail, "tail", 0, "Last 'n' logs and events to fetch. By default it will pull everything")
	hcpMgCmd.Flags().StringVar(&g.SortOrder, "sort", "asc", "Sort the results by timestamp in either ascending or descending order. Accepted values are 'asc' and 'desc'")
	hcpMgCmd.Flags().StringVar(&g.DestDir, "dest-dir", "", "Destination directory for the logs dump, defaults to the local directory.")
	hcpMgCmd.Flags().StringVar(&g.ClusterID, "cluster-id", "", "Internal ID of the HCP cluster to gather logs from (required)")

	_ = hcpMgCmd.MarkFlagRequired("cluster-id")

	return hcpMgCmd
}

func (g *GatherLogsOpts) GatherLogs(clusterID string) (error error) {
	accessToken, err := getStorageAccessToken()
	if err != nil {
		return fmt.Errorf("failed to acquire access token %v", err)
	}

	hcpCluster, err := FetchClusterDetails(clusterID)
	if err != nil {
		return err
	}

	_, _, clientset, err := common.GetKubeConfigAndClient(hcpCluster.managementClusterID, "", "")
	if err != nil {
		return fmt.Errorf("failed to retrieve Kubernetes configuration and client for cluster with ID %s: %w", hcpCluster.managementClusterID, err)
	}

	fmt.Printf("Using HCP Namespace %v\n", hcpCluster.hcpNamespace)

	gatherNamespaces := []string{hcpCluster.hcpNamespace, hcpCluster.klusterletNS, hcpCluster.hostedNS, "hypershift", "cert-manager", "redhat-cert-manager-operator", "open-cluster-management-agent", "open-cluster-management-agent-addon"}

	gatherDir, err := setupGatherDir(g.DestDir, hcpCluster.hcpNamespace)
	if err != nil {
		return err
	}

	for _, gatherNS := range gatherNamespaces {
		fmt.Printf("Gathering for %s\n", gatherNS)

		pods, err := getPodsForNamespace(clientset, gatherNS)
		if err != nil {
			return err
		}

		nsDir, err := addDir([]string{gatherDir, gatherNS}, []string{})
		if err != nil {
			return err
		}

		err = g.dumpPodLogs(pods, nsDir, gatherNS, hcpCluster.managementClusterName, hcpCluster.DynatraceURL, accessToken, g.Since, g.Tail, g.SortOrder)
		if err != nil {
			return err
		}

		deployments, err := getDeploymentsForNamespace(clientset, gatherNS)
		if err != nil {
			return err
		}

		err = g.dumpEvents(deployments, nsDir, gatherNS, hcpCluster.managementClusterName, hcpCluster.DynatraceURL, accessToken, g.Since, g.Tail, g.SortOrder)
		if err != nil {
			return err
		}

		err = g.dumpRestartedPodLogs(pods, nsDir, gatherNS, hcpCluster.managementClusterName, hcpCluster.DynatraceURL, accessToken)
		if err != nil {
			return err
		}

	}

	return nil
}

func (g *GatherLogsOpts) dumpEvents(deploys *appsv1.DeploymentList, parentDir string, targetNS string, managementClusterName string, DTURL string, accessToken string, since int, tail int, sortOrder string) error {
	totalDeployments := len(deploys.Items)
	for k, d := range deploys.Items {
		fmt.Printf("[%d/%d] Deployment events for %s\n", k+1, totalDeployments, d.Name)

		eventQuery, err := getEventQuery(d.Name, targetNS, g.Since, g.Tail, g.SortOrder, managementClusterName)
		if err != nil {
			return err
		}
		eventQuery.Build()

		deploymentYamlFileName := "deployment.yaml"
		eventsFileName := "events.log"
		eventsDirPath, err := addDir([]string{parentDir, "events", d.Name}, []string{deploymentYamlFileName, eventsFileName})
		if err != nil {
			return err
		}

		deploymentYamlPath := filepath.Join(eventsDirPath, deploymentYamlFileName)
		deploymentYaml, err := yaml.Marshal(d)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %v", err)
		}
		f, err := os.OpenFile(deploymentYamlPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(deploymentYaml)
		if err != nil {
			return err
		}
		err = f.Close()
		if err != nil {
			return err
		}

		eventsFilePath := filepath.Join(eventsDirPath, eventsFileName)
		f, err = os.OpenFile(eventsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		eventsRequestToken, err := getDTQueryExecution(DTURL, accessToken, eventQuery.finalQuery)
		if err != nil {
			log.Print("failed to get request token", err)
			continue
		}
		err = getEvents(DTURL, accessToken, eventsRequestToken, f)
		_ = f.Close()
		if err != nil {
			log.Printf("failed to get logs, continuing: %v. Query: %v", err, eventQuery.finalQuery)
			continue
		}

	}
	return nil
}

func (g *GatherLogsOpts) dumpPodLogs(pods *corev1.PodList, parentDir string, targetNS string, managementClusterName string, DTURL string, accessToken string, since int, tail int, sortOrder string) error {
	totalPods := len(pods.Items)
	for k, p := range pods.Items {
		fmt.Printf("[%d/%d] Pod logs for %s\n", k+1, totalPods, p.Name)

		podLogsQuery, err := getPodQuery(p.Name, targetNS, g.Since, g.Tail, g.SortOrder, managementClusterName)
		if err != nil {
			return err
		}
		podLogsQuery.Build()

		podYamlFileName := "pod.yaml"
		podLogFileName := "pod.log"
		podDirPath, err := addDir([]string{parentDir, "pods", p.Name}, []string{podLogFileName, podYamlFileName})
		if err != nil {
			return err
		}

		podYamlFilePath := filepath.Join(podDirPath, podYamlFileName)
		podYaml, err := yaml.Marshal(p)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %v", err)
		}
		f, err := os.OpenFile(podYamlFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		_, err = f.Write(podYaml)
		if err != nil {
			return err
		}
		_ = f.Close()

		podLogsFilePath := filepath.Join(podDirPath, podLogFileName)
		f, err = os.OpenFile(podLogsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}

		podLogsRequestToken, err := getDTQueryExecution(DTURL, accessToken, podLogsQuery.finalQuery)
		if err != nil {
			log.Print("failed to get request token", err)
			continue
		}
		err = getLogs(DTURL, accessToken, podLogsRequestToken, f)
		_ = f.Close()
		if err != nil {
			log.Printf("failed to get logs, continuing: %v. Query: %v", err, podLogsQuery.finalQuery)
			continue
		}
	}

	return nil
}

func (g *GatherLogsOpts) dumpRestartedPodLogs(pods *corev1.PodList, parentDir string, targetNS string, managementClusterName string, DTURL string, accessToken string) error {
	var podList []string
	for _, p := range pods.Items {
		podList = append(podList, p.Name)
	}
	fmt.Printf("Collecting Restarted Pod logs for %s\n", targetNS)

	restartedPodLogsQuery, err := getRestartedPodQuery(podList, targetNS, g.Since, g.Tail, g.SortOrder, managementClusterName)
	if err != nil {
		return err
	}
	restartedPodLogsQuery.Build()

	restartedPodLogFileName := "pods.log"
	podDirPath, err := addDir([]string{parentDir, "restarted-pods"}, []string{restartedPodLogFileName})
	if err != nil {
		return err
	}

	restartedPodLogsFilePath := filepath.Join(podDirPath, restartedPodLogFileName)
	f, err := os.OpenFile(restartedPodLogsFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0655)
	if err != nil {
		return err
	}

	podLogsRequestToken, err := getDTQueryExecution(DTURL, accessToken, restartedPodLogsQuery.finalQuery)
	if err != nil {
		log.Print("failed to get request token", err)

	}
	err = getLogs(DTURL, accessToken, podLogsRequestToken, f)
	f.Close()
	if err != nil {
		log.Printf("failed to get restarted pod logs: %v. Query: %v", err, restartedPodLogsQuery.finalQuery)
	}

	return nil
}

func setupGatherDir(destBaseDir string, dirName string) (logsDir string, error error) {
	dirPath := filepath.Join(destBaseDir, fmt.Sprintf("hcp-logs-dump-%s", dirName))
	err := os.MkdirAll(dirPath, 0750)
	if err != nil {
		return "", fmt.Errorf("failed to setup logs directory %v", err)
	}

	return dirPath, nil
}

func addDir(dirs []string, filePaths []string) (path string, error error) {
	dirPath := filepath.Join(dirs...)
	err := os.MkdirAll(dirPath, 0750)
	if err != nil {
		return "", fmt.Errorf("failed to setup directory %v", err)
	}
	for _, fp := range filePaths {
		createdFile := filepath.Join(dirPath, fp)
		_, err = os.Create(createdFile)
		if err != nil {
			return "", fmt.Errorf("file to create file %v in %v", fp, err)
		}
	}

	return dirPath, nil
}

func getPodQuery(pod string, namespace string, since int, tail int, sortOrder string, srcCluster string) (query DTQuery, error error) {
	q := DTQuery{}
	q.InitLogs(since).Cluster(srcCluster)

	if namespace != "" {
		q.Namespaces([]string{namespace})
	}

	if pod != "" {
		q.Pods([]string{pod})
	}

	if sortOrder != "" {
		q, err := q.Sort(sortOrder)
		if err != nil {
			return *q, err
		}
	}

	if tail > 0 {
		q.Limit(tail)
	}

	return q, nil
}

func getRestartedPodQuery(pods []string, namespace string, since int, tail int, sortOrder string, srcCluster string) (query DTQuery, error error) {
	q := DTQuery{}
	q.InitLogs(since).Cluster(srcCluster)

	if namespace != "" {
		q.Namespaces([]string{namespace})
	}

	if len(pods) > 0 {
		q.Pods(pods)
		for i := 0; i < len(q.fragments); i++ {
			if strings.Contains(q.fragments[i], "k8s.pod.name") {
				q.fragments[i] = strings.Replace(q.fragments[i], "and (", "and not (", 1)
				break
			}
		}
	}

	if sortOrder != "" {
		q, err := q.Sort(sortOrder)
		if err != nil {
			return *q, err
		}
	}

	if tail > 0 {
		q.Limit(tail)
	}

	return q, nil
}

func getEventQuery(deploy string, namespace string, since int, tail int, sortOrder string, srcCluster string) (query DTQuery, error error) {
	q := DTQuery{}
	q.InitEvents(since).Cluster(srcCluster)

	if namespace != "" {
		q.Namespaces([]string{namespace})
	}

	if deploy != "" {
		q.Deployments([]string{deploy})
	}

	if sortOrder != "" {
		q, err := q.Sort(sortOrder)
		if err != nil {
			return *q, err
		}
	}

	if tail > 0 {
		q.Limit(tail)
	}

	return q, nil
}

func getPodsForNamespace(clientset *kubernetes.Clientset, namespace string) (pl *corev1.PodList, error error) {
	// Getting pod objects for non-running state pod
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace '%s'", namespace)
	}

	return pods, nil
}

func getDeploymentsForNamespace(clientset *kubernetes.Clientset, namespace string) (pl *appsv1.DeploymentList, error error) {
	// Getting pod objects for non-running state pod
	deploys, err := clientset.AppsV1().Deployments(namespace).List(context.TODO(), v1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace '%s'", namespace)
	}

	return deploys, nil
}
