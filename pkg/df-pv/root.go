package df_pv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	// "github.com/fatih/color"
	// "github.com/gookit/color"
	// . "github.com/logrusorgru/aurora"
	// "k8s.io/cli-runtime/pkg/printers"
	// "github.com/olekukonko/tablewriter"
	"github.com/jedib0t/go-pretty/table"
	"github.com/jedib0t/go-pretty/text"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// InitAndExecute sets up and executes the cobra root command
func InitAndExecute() {
	rootCmd := setupRootCommand()
	if err := errors.Wrapf(rootCmd.Execute(), "run df-pv root command"); err != nil {
		log.Fatalf("unable to run root command: %+v", err)
		os.Exit(1)
	}
}

type flagpole struct {
	logLevel              string
	genericCliConfigFlags *genericclioptions.ConfigFlags
	disableColor          bool
}

func setupRootCommand() *cobra.Command {
	flags := &flagpole{}
	var rootCmd = &cobra.Command{
		Use:   "df-pv",
		Short: "df-pv emulates Unix style df for persistent volumes",
		Long: `df-pv emulates Unix style df for persistent volumes w/ ability to filter by namespace

It autoconverts all "sizes" to IEC values (see: https://en.wikipedia.org/wiki/Binary_prefix and https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory)

It colors the values based on "severity" [red: > 75% (too high); yellow: < 25% (too low); green: >= 25 and <= 75 (OK)]`,
		Args: cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRootCommand(flags)
		},
	}

	rootCmd.PersistentFlags().StringVarP(&flags.logLevel, "verbosity", "v", "info", "log level; one of [info, debug, trace, warn, error, fatal, panic]")
	rootCmd.Flags().BoolVarP(&flags.disableColor, "disable-color", "d", false, "boolean flag for disabling colored output")

	flags.genericCliConfigFlags = genericclioptions.NewConfigFlags(false)
	flags.genericCliConfigFlags.AddFlags(rootCmd.Flags())

	return rootCmd
}

func runRootCommand(flags *flagpole) error {
	logLevel, _ := log.ParseLevel(flags.logLevel)
	log.SetLevel(logLevel)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})

	sliceOfOutputRowPVC, err := GetSliceOfOutputRowPVC(flags)
	if err != nil {
		return errors.Wrapf(err, "error getting output slice")
	}

	if nil == sliceOfOutputRowPVC || 0 > len(sliceOfOutputRowPVC) {
		// ns := flags.namespace
		ns := *flags.genericCliConfigFlags.Namespace
		if 0 == len(ns) {
			ns = "all"
		}
		log.Infof("Either no volumes found in namespace/s: '%s' or the storage provisioner used for the volumes does not publish metrics to kubelet", ns)
	} else {
		PrintUsingGoPretty(sliceOfOutputRowPVC, flags.disableColor)
	}

	return nil
}

// PrintUsingGoPretty prints a slice of output rows
func PrintUsingGoPretty(sliceOfOutputRowPVC []*OutputRowPVC, disableColor bool) {
	if disableColor {
		text.DisableColors()
	}
	// https://github.com/jedib0t/go-pretty/tree/v6.0.4/table
	t := table.NewWriter()

	t.AppendHeader(table.Row{"PV Name", "PVC Name", "Namespace", "Node Name", "Pod Name", "Volume Mount Name", "Size", "Used", "Available", "%Used", "iused", "ifree", "%iused"})
	var whiteColor = text.FgWhite
	var percentageUsedColor text.Color
	var percentageIUsedColor text.Color
	for _, pvcRow := range sliceOfOutputRowPVC {
		percentageUsedColor = GetColorFromPercentageUsed(pvcRow.PercentageUsed)
		percentageIUsedColor = GetColorFromPercentageUsed(pvcRow.PercentageIUsed)
		t.AppendRow([]interface{}{
			fmt.Sprintf("%s", pvcRow.PVName),
			fmt.Sprintf("%s", pvcRow.PVCName),
			fmt.Sprintf("%s", pvcRow.Namespace),
			fmt.Sprintf("%s", pvcRow.NodeName),
			fmt.Sprintf("%s", pvcRow.PodName),
			fmt.Sprintf("%s", pvcRow.VolumeMountName),
			percentageUsedColor.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.CapacityBytes)),
			percentageUsedColor.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.UsedBytes)),
			percentageUsedColor.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.AvailableBytes)),
			percentageUsedColor.Sprintf("%.2f", pvcRow.PercentageUsed),
			percentageIUsedColor.Sprintf("%d", pvcRow.InodesUsed),
			percentageIUsedColor.Sprintf("%d", pvcRow.InodesFree),
			percentageIUsedColor.Sprintf("%.2f", pvcRow.PercentageIUsed),
		})
	}

	// https://github.com/jedib0t/go-pretty/blob/v6.0.4/table/style.go
	styleBold := table.StyleBold
	styleBold.Options = table.OptionsNoBordersAndSeparators
	t.SetStyle(styleBold)
	t.Style().Color.Header = text.Colors{whiteColor, text.Bold}
	// t.Style().Options.SeparateRows = true
	// t.SetAutoIndex(true)
	// t.SetOutputMirror(os.Stdout)
	fmt.Printf("\n%s\n\n", t.Render())
}

// GetColorFromPercentageUsed gives a color based on percentage
func GetColorFromPercentageUsed(percentageUsed float64) text.Color {
	if percentageUsed > 75 {
		return text.FgRed
	} else if percentageUsed < 25 {
		return text.FgGreen
	} else {
		return text.FgYellow
	}
}

// ConvertQuantityValueToHumanReadableIECString converts value to human readable IEC format
// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
func ConvertQuantityValueToHumanReadableIECString(quantity *resource.Quantity) string {
	var val = quantity.Value()
	var suffix string

	// https://en.wikipedia.org/wiki/Tebibyte
	// 1 TiB = 2^40 bytes = 1099511627776 bytes = 1024 gibibytes
	TiConvertedVal := val / 1099511627776
	// https://en.wikipedia.org/wiki/Gibibyte
	// 1 GiB = 2^30 bytes = 1073741824 bytes = 1024 mebibytes
	GiConvertedVal := val / 1073741824
	// https://en.wikipedia.org/wiki/Mebibyte
	// 1 MiB = 2^20 bytes = 1048576 bytes = 1024 kibibytes
	MiConvertedVal := val / 1048576
	// https://en.wikipedia.org/wiki/Kibibyte
	// 1 KiB = 2^10 bytes = 1024 bytes
	KiConvertedVal := val / 1024

	if 1 < TiConvertedVal {
		suffix = "Ti"
		return fmt.Sprintf("%d%s", TiConvertedVal, suffix)
	} else if 1 < GiConvertedVal {
		suffix = "Gi"
		return fmt.Sprintf("%d%s", GiConvertedVal, suffix)
	} else if 1 < MiConvertedVal {
		suffix = "Mi"
		return fmt.Sprintf("%d%s", MiConvertedVal, suffix)
	} else if 1 < KiConvertedVal {
		suffix = "Ki"
		return fmt.Sprintf("%d%s", KiConvertedVal, suffix)
	} else {
		return fmt.Sprintf("%d", val)
	}
}

// ConvertQuantityValueToHumanReadableDecimalString converts value to human readable decimal format
func ConvertQuantityValueToHumanReadableDecimalString(quantity *resource.Quantity) string {
	var val = quantity.Value()
	var suffix string

	TBConvertedVal := val / 1000000000000
	GBConvertedVal := val / 1000000000
	MBConvertedVal := val / 1000000
	KBConvertedVal := val / 1000

	if 1 < TBConvertedVal {
		suffix = "TB"
		return fmt.Sprintf("%d%s", TBConvertedVal, suffix)
	} else if 1 < GBConvertedVal {
		suffix = "GB"
		return fmt.Sprintf("%d%s", GBConvertedVal, suffix)
	} else if 1 < MBConvertedVal {
		suffix = "MB"
		return fmt.Sprintf("%d%s", MBConvertedVal, suffix)
	} else if 1 < KBConvertedVal {
		suffix = "KB"
		return fmt.Sprintf("%d%s", KBConvertedVal, suffix)
	} else {
		return fmt.Sprintf("%d", val)
	}
}

// OutputRowPVC represents the output row
type OutputRowPVC struct {
	PVName          string             `json:"pvName"`
	PVCName         string             `json:"pvcName"`
	Namespace       string             `json:"namespace"`
	NodeName        string             `json:"nodeName"`
	PodName         string             `json:"podName"`
	VolumeMountName string             `json:"volumeMountName"`
	AvailableBytes  *resource.Quantity `json:"availableBytes"` // TODO: use uint64 here as well? but resource.Quantity takes int64
	CapacityBytes   *resource.Quantity `json:"capacityBytes"`
	UsedBytes       *resource.Quantity `json:"usedBytes"`
	InodesFree      uint64             `json:"inodesFree"`
	Inodes          uint64             `json:"inodes"`
	InodesUsed      uint64             `json:"inodesUsed"`
	PercentageUsed  float64            `json:"percentageUsed"`
	PercentageIUsed float64            `json:"percentageIUsed"`
}

// ServerResponseStruct represents the response at the node endpoint
type ServerResponseStruct struct {
	Pods []*Pod `json:"pods"`
}

// Pod represents pod spec in the server response
type Pod struct {
	/*
		EXAMPLE:
		"podRef": {
		     "name": "configs-service-59c9c7586b-5jchj",
		     "namespace": "onprem",
		     "uid": "5fbb63da-d0a3-4493-8d27-6576b63119f5"
		    }
	*/
	PodRef struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"podRef"`
	/*
		EXAMPLE:
		"volume": [
		     {...},
		     {...}
		    ]
	*/
	ListOfVolumes []*Volume `json:"volume"`
}

// Volume represents the volume struct
/*
EXAMPLE:
{
"time": "2019-11-25T20:33:19Z",
"availableBytes": 25674719232,
"capacityBytes": 25674731520,
"usedBytes": 12288,
"inodesFree": 6268236,
"inodes": 6268245,
"inodesUsed": 9,
"name": "vault-client"
}
*/
// https://github.com/kubernetes/kubernetes/blob/v1.18.5/pkg/volume/volume.go
// https://github.com/kubernetes/kubernetes/blob/v1.18.5/pkg/volume/csi/csi_client.go#L553
type Volume struct {
	// The time at which these stats were updated.
	Time metav1.Time `json:"time"`

	// Used represents the total bytes used by the Volume.
	// Note: For block devices this maybe more than the total size of the files.
	UsedBytes int64 `json:"usedBytes"` // TODO: use uint64 here as well?

	// Capacity represents the total capacity (bytes) of the volume's
	// underlying storage. For Volumes that share a filesystem with the host
	// (e.g. emptydir, hostpath) this is the size of the underlying storage,
	// and will not equal Used + Available as the fs is shared.
	CapacityBytes int64 `json:"capacityBytes"`

	// Available represents the storage space available (bytes) for the
	// Volume. For Volumes that share a filesystem with the host (e.g.
	// emptydir, hostpath), this is the available space on the underlying
	// storage, and is shared with host processes and other Volumes.
	AvailableBytes int64 `json:"availableBytes"`

	// InodesUsed represents the total inodes used by the Volume.
	InodesUsed uint64 `json:"inodesUsed"`

	// Inodes represents the total number of inodes available in the volume.
	// For volumes that share a filesystem with the host (e.g. emptydir, hostpath),
	// this is the inodes available in the underlying storage,
	// and will not equal InodesUsed + InodesFree as the fs is shared.
	Inodes uint64 `json:"inodes"`

	// InodesFree represent the inodes available for the volume.  For Volumes that share
	// a filesystem with the host (e.g. emptydir, hostpath), this is the free inodes
	// on the underlying storage, and is shared with host processes and other volumes
	InodesFree uint64 `json:"inodesFree"`

	Name   string `json:"name"`
	PvcRef struct {
		PvcName      string `json:"name"`
		PvcNamespace string `json:"namespace"`
	} `json:"pvcRef"`
}

// GetSliceOfOutputRowPVC gets the output row
func GetSliceOfOutputRowPVC(flags *flagpole) ([]*OutputRowPVC, error) {

	ctx := context.Background()

	kubeConfig, err := GetKubeConfigFromGenericCliConfigFlags(flags.genericCliConfigFlags)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to build config from flags")
	}

	// kubeConfigPath, err := KubeConfigPath()
	// if err != nil {
	// 	return nil, err
	// }
	//
	// log.Debugf("instantiating k8s client from config path: '%s'", kubeConfigPath)
	// kubeConfig, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	// // kubeConfig, err := rest.InClusterConfig()
	// if err != nil {
	// 	return nil, errors.Wrapf(err, "unable to build config from flags")
	// }

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create clientset")
	}

	desiredNamespace := *flags.genericCliConfigFlags.Namespace
	// desiredNamespace := flags.namespace
	var sliceOfNodeName []string
	if 0 == len(desiredNamespace) {
		nodes, err := ListNodes(ctx, clientset)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to list nodes")
		}
		nodeItems := nodes.Items
		for _, node := range nodeItems {
			sliceOfNodeName = append(sliceOfNodeName, node.Name)
		}
	} else {
		nodeNameToPodNames, err := GetWhichNodesToQueryBasedOnNamespace(ctx, clientset, desiredNamespace)
		if err != nil {
			return nil, err
		}
		for nodeName := range nodeNameToPodNames {
			sliceOfNodeName = append(sliceOfNodeName, nodeName)
		}
	}

	var mainGroup run.Group
	outputRowPVCChan := make(chan *OutputRowPVC)
	var sliceOfOutputRowPVC []*OutputRowPVC

	// produce concurrently
	{
		mainGroup.Add(func() error {
			return ProduceOutputRowsConcurrently(ctx, clientset, desiredNamespace, sliceOfNodeName, outputRowPVCChan)
		}, func(err error) {
			if err != nil {
				log.Infof("TODO goroutine error handling; current actor was interrupted with: %v\n", err)
			}
		})
	}

	// consume concurrently
	{
		mainGroup.Add(func() error {
			sliceOfOutputRowPVC = ConsumeOutputRowsConcurrently(outputRowPVCChan)
			return nil
		}, func(err error) {
			if err != nil {
				log.Infof("TODO goroutine error handling; current actor was interrupted with: %v\n", err)
			}
		})
	}

	return sliceOfOutputRowPVC, mainGroup.Run()
}

// ConsumeOutputRowsConcurrently consumes processed output rows concurrently
func ConsumeOutputRowsConcurrently(outputRowPVCChan <-chan *OutputRowPVC) []*OutputRowPVC {
	var sliceOfOutputRowPVC []*OutputRowPVC
	for outputRowPVC := range outputRowPVCChan {
		sliceOfOutputRowPVC = append(sliceOfOutputRowPVC, outputRowPVC)
	}
	return sliceOfOutputRowPVC
}

// ProduceOutputRowsConcurrently produces output rows concurrently
func ProduceOutputRowsConcurrently(ctx context.Context, clientset *kubernetes.Clientset, desiredNamespace string, nodeNames []string, outputRowPVCChan chan<- *OutputRowPVC) error {
	var producerGroup run.Group
	for _, nodeName := range nodeNames {
		nodeName := nodeName
		producerGroup.Add(func() error {
			return GetOutputRowPVCFromNode(ctx, clientset, desiredNamespace, nodeName, outputRowPVCChan)
		}, func(err error) {
			if err != nil {
				log.Infof("TODO goroutine error handling; current actor was interrupted with: %v\n", err)
			}
		})
	}
	if err := producerGroup.Run(); err != nil {
		return err
	}
	close(outputRowPVCChan)
	return nil
}

// GetOutputRowPVCFromNode gets the output row given a nodeName
func GetOutputRowPVCFromNode(ctx context.Context, clientset *kubernetes.Clientset, desiredNamespace string, nodeName string, outputRowPVCChan chan<- *OutputRowPVC) error {
	log.Tracef("connecting to node: %s", nodeName)
	request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(nodeName).SubResource("proxy").Suffix("stats/summary")
	res := request.Do(ctx)

	responseRawArrayOfBytes, err := res.Raw()
	if err != nil {
		return errors.Wrapf(err, "failed to get stats from node")
	}

	// for trace logging only
	var nodeRespBody interface{}
	err = json.Unmarshal(responseRawArrayOfBytes, &nodeRespBody)
	if err != nil {
		return errors.Wrapf(err, "unable to unmarshal json into an interface (this really shouldn't happen)")
	}
	// log.Tracef("response from node: %+v\n", nodeRespBody)
	jsonText, err := json.Marshal(nodeRespBody)
	// jsonText, err := json.MarshalIndent(nodeRespBody, "", "  ")
	if err != nil {
		return errors.Wrapf(err, "unable to marshal json (this really shouldn't happen)")
	}
	log.Tracef("response from node: %s", jsonText)

	var jsonConvertedIntoStruct ServerResponseStruct
	err = json.Unmarshal(responseRawArrayOfBytes, &jsonConvertedIntoStruct)
	if err != nil {
		return errors.Wrapf(err, "failed to convert the response from server")
	}

	for _, pod := range jsonConvertedIntoStruct.Pods {
		for _, vol := range pod.ListOfVolumes {
			outputRowPVC := GetOutputRowPVCFromPodAndVolume(ctx, clientset, pod, vol, desiredNamespace)
			if nil == outputRowPVC {
				log.Tracef("no pvc found for pod: '%s', vol: '%s', desiredNamespace: '%s'; continuing...", pod.PodRef.Name, vol.PvcRef.PvcName, desiredNamespace)
				continue
			}
			outputRowPVC.NodeName = nodeName
			select {
			case <-ctx.Done():
				return ctx.Err()
			case outputRowPVCChan <- outputRowPVC:
				log.Debugf("Got metrics for pvc '%s' from node: '%s'", outputRowPVC.PVCName, nodeName)
			}
		}
	}
	return nil
}

// GetWhichNodesToQueryBasedOnNamespace gets a list of nodes to query for all the pods in a namespace
func GetWhichNodesToQueryBasedOnNamespace(ctx context.Context, clientset *kubernetes.Clientset, desiredNamespace string) (map[string][]string, error) {
	sliceOfPod, err := ListPodsWithPersistentVolumeClaims(ctx, clientset, desiredNamespace)
	if err != nil {
		return nil, err
	}

	nodeNameToPodNames := make(map[string][]string)
	for _, pod := range sliceOfPod {
		nodeName := pod.Spec.NodeName
		podName := pod.Name
		nodeNameToPodNames[nodeName] = append(nodeNameToPodNames[nodeName], podName)
	}
	return nodeNameToPodNames, nil
}

// GetOutputRowPVCFromPodAndVolume gets an output row for a given pod, volume and optionally namespace
func GetOutputRowPVCFromPodAndVolume(ctx context.Context, clientset *kubernetes.Clientset, pod *Pod, vol *Volume, desiredNamespace string) *OutputRowPVC {
	var outputRowPVC *OutputRowPVC

	if 0 < len(desiredNamespace) {
		if vol.PvcRef.PvcNamespace != desiredNamespace {
			return nil
		}
	}
	log.Debugf("restricting findings to namespace: '%s'", desiredNamespace)

	if 0 < len(vol.PvcRef.PvcName) {
		namespace := pod.PodRef.Namespace
		pvcName := vol.PvcRef.PvcName
		pvName, _ := GetPVNameFromPVCName(ctx, clientset, namespace, pvcName)
		// if err != nil {
		// 	ctx.Err()
		// 	return errors.Wrapf(err, "unable to get PV name from the PVC name")
		// }

		outputRowPVC = &OutputRowPVC{
			Namespace:       namespace,
			PVCName:         pvcName,
			PVName:          pvName,
			PodName:         pod.PodRef.Name,
			VolumeMountName: vol.Name,
			AvailableBytes:  resource.NewQuantity(vol.AvailableBytes, resource.BinarySI),
			CapacityBytes:   resource.NewQuantity(vol.CapacityBytes, resource.BinarySI),
			UsedBytes:       resource.NewQuantity(vol.UsedBytes, resource.BinarySI),
			PercentageUsed:  (float64(vol.UsedBytes) / float64(vol.CapacityBytes)) * 100.0,
			Inodes:          vol.Inodes,
			InodesFree:      vol.InodesFree,
			InodesUsed:      vol.InodesUsed,
			PercentageIUsed: (float64(vol.InodesUsed) / float64(vol.Inodes)) * 100.0,
		}
	}
	return outputRowPVC
}

// GetKubeConfigFromGenericCliConfigFlags gets the kubeconfig from all the flags
func GetKubeConfigFromGenericCliConfigFlags(genericCliConfigFlags *genericclioptions.ConfigFlags) (*rest.Config, error) {
	config, err := genericCliConfigFlags.ToRESTConfig()
	return config, errors.Wrap(err, "failed to read kubeconfig")
}

// ListNodes returns a list of nodes
func ListNodes(ctx context.Context, clientset *kubernetes.Clientset) (*corev1.NodeList, error) {
	log.Tracef("getting a list of all nodes")
	return clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

// ListPods returns a list of pods
func ListPods(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (*corev1.PodList, error) {
	log.Tracef("getting a list of all pods in namespace: %s", namespace)
	return clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
}

// ListPodsWithPersistentVolumeClaims returns a list of pods with PVCs
// kubectl get pods --all-namespaces -o=json | jq -c \
// '.items[] | {name: .metadata.name, namespace: .metadata.namespace, claimName:.spec.volumes[] | select( has ("persistentVolumeClaim") ).persistentVolumeClaim.claimName }'
func ListPodsWithPersistentVolumeClaims(ctx context.Context, clientset *kubernetes.Clientset, namespace string) ([]corev1.Pod, error) {
	log.Tracef("getting a list of pods with PVC in namespace: %s", namespace)
	pods, err := ListPods(ctx, clientset, namespace)
	if err != nil {
		return nil, err
	}
	var sliceOfPodsWithPVCs []corev1.Pod
	for _, pod := range pods.Items {
		volumes := pod.Spec.Volumes
		for _, vol := range volumes {
			if vol.PersistentVolumeClaim != nil && 0 < len(vol.PersistentVolumeClaim.ClaimName) {
				log.Tracef("found pod: %s with PVC: %s", pod.Name, vol.PersistentVolumeClaim.ClaimName)
				sliceOfPodsWithPVCs = append(sliceOfPodsWithPVCs, pod)
			}
		}
	}
	return sliceOfPodsWithPVCs, err
}

// GetPVNameFromPVCName returns the name of persistent volume given a namespace and persistent volume claim name
func GetPVNameFromPVCName(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pvcName string) (string, error) {
	var pvName string
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return pvName, err
	}
	pvName = pvc.Spec.VolumeName
	return pvName, err
}

// KubeConfigPath returns the path to kubeconfig file
func KubeConfigPath() (string, error) {
	log.Debugf("getting kubeconfig path based on user's home dir")
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get home dir")
	}
	return path.Join(home, ".kube", "config"), nil
}

// ListPVCs returns a list of PVCs for a given namespace
func ListPVCs(ctx context.Context, clientset *kubernetes.Clientset, namespace string) (*corev1.PersistentVolumeClaimList, error) {
	log.Tracef("getting a list of all PVCs in namespace: %s", namespace)
	return clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
}

// ListPVs returns a list of PVs, scoped to corresponding mapped PVCs based on the namespace
func ListPVs(ctx context.Context, clientset *kubernetes.Clientset, namespace string) {
	pvList, _ := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	var pvcNames []string
	for _, pv := range pvList.Items {
		if pv.Spec.ClaimRef.Name == namespace {
			pvcName := pv.Spec.ClaimRef.Name
			pvcNames = append(pvcNames, pvcName)
		}
	}
}
