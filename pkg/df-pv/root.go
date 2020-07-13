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
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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
	// namespace             string
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

	// rootCmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "if present, the namespace scope for this CLI request (default is all namespaces)")
	rootCmd.PersistentFlags().StringVarP(&flags.logLevel, "verbosity", "v", "info", "log level; one of [info, debug, trace, warn, error, fatal, panic]")

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
		PrintUsingGoPretty(sliceOfOutputRowPVC)
	}

	return nil
}

func PrintUsingGoPretty(sliceOfOutputRowPVC []*OutputRowPVC) {
	// https://github.com/jedib0t/go-pretty/tree/v6.0.4/table
	t := table.NewWriter()

	t.AppendHeader(table.Row{"Namespace", "PVC Name", "PV Name", "Pod Name", "Volume Mount Name", "Size", "Used", "Available", "%Used", "iused", "ifree", "%iused"})
	hiWhiteColor := text.FgHiWhite
	for _, pvcRow := range sliceOfOutputRowPVC {
		percentageUsedColor := GetColorFromPercentageUsed(pvcRow.PercentageUsed)
		percentageIUsedColor := GetColorFromPercentageUsed(pvcRow.PercentageIUsed)
		t.AppendRow([]interface{}{
			hiWhiteColor.Sprintf("%s", pvcRow.Namespace),
			hiWhiteColor.Sprintf("%s", pvcRow.PVCName),
			hiWhiteColor.Sprintf("%s", pvcRow.PVName),
			hiWhiteColor.Sprintf("%s", pvcRow.PodName),
			hiWhiteColor.Sprintf("%s", pvcRow.VolumeMountName),
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
	t.Style().Color.Header = text.Colors{hiWhiteColor, text.Bold}
	// t.Style().Options.SeparateRows = true
	// t.SetAutoIndex(true)
	// t.SetOutputMirror(os.Stdout)
	fmt.Printf("\n%s\n\n", t.Render())
}

func GetColorFromPercentageUsed(percentageUsed float64) text.Color {
	if percentageUsed > 75 {
		return text.FgHiRed
	} else if percentageUsed < 25 {
		return text.FgHiYellow
	} else {
		return text.FgHiGreen
	}
}

// // TODO: this tablewriter doesn't allow changing color per row based on the value
// func PrintUsingTableWriter(sliceOfOutputRowPVC []*OutputRowPVC) {
//
// 	var data [][]string
// 	for _, pvcRow := range sliceOfOutputRowPVC {
// 		currData := []string{
// 			fmt.Sprintf("%s", pvcRow.PVCName),
// 			fmt.Sprintf("%s", pvcRow.Namespace),
// 			fmt.Sprintf("%s", pvcRow.PodName),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.CapacityBytes)),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.UsedBytes)),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.AvailableBytes)),
// 			fmt.Sprintf("%.2f", pvcRow.PercentageUsed),
// 			fmt.Sprintf("%d", pvcRow.InodesUsed),
// 			fmt.Sprintf("%d", pvcRow.InodesFree),
// 			fmt.Sprintf("%.2f", pvcRow.PercentageIUsed),
// 		}
// 		data = append(data, currData)
// 	}
//
// 	table := tablewriter.NewWriter(os.Stdout)
// 	table.SetHeader([]string{"PVC", "Namespace", "Pod", "Size", "Used", "Available", "PercentUsed", "iused", "ifree", "Percentiused"})
//
// 	table.SetAutoWrapText(false)
// 	table.SetAutoFormatHeaders(true)
// 	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
// 	table.SetAlignment(tablewriter.ALIGN_LEFT)
// 	table.SetCenterSeparator("")
// 	table.SetColumnSeparator("")
// 	table.SetRowSeparator("")
// 	table.SetHeaderLine(false)
// 	table.SetBorder(false)
// 	table.SetTablePadding("\t") // pad with tabs
// 	table.SetNoWhiteSpace(true)
//
// 	table.SetColumnColor()
//
// 	table.AppendBulk(data) // Add Bulk Data
// 	table.Render()
// }
//
// // TODO: color messes up the formatting, but this potentially will have sorting some day (currently it still does not)
// func PrintTableUsingKubeTable(sliceOfOutputRowPVC []*OutputRowPVC) {
// 	var columns = []metav1.TableColumnDefinition{
// 		{Name: "PVC", Type: "string"},
// 		{Name: "Namespace", Type: "string"},
// 		{Name: "Pod", Type: "string"},
// 		{Name: "Size", Type: "string"},
// 		{Name: "Used", Type: "string"},
// 		{Name: "Available", Type: "string"},
// 		{Name: "PercentUsed", Type: "number", Format: "float"},
// 		{Name: "iused", Type: "integer", Format: "int32"},
// 		{Name: "ifree", Type: "integer", Format: "int32"},
// 		{Name: "Percentiused", Type: "number", Format: "float"},
// 	}
// 	var rows []metav1.TableRow
//
// 	colorFmt := color.New()
//
// 	for _, pvcRow := range sliceOfOutputRowPVC {
// 		colorFmt = color.Style{color.FgWhite}
// 		pvcPercentageUsedVal := pvcRow.PercentageUsed
// 		// pvcPercentageUsedString := fmt.Sprintf("\033[31m%.2f\033[0m", pvcPercentageUsedVal)
//
// 		if pvcPercentageUsedVal > 75 {
// 			colorFmt = color.Style{color.Red}
// 		} else if pvcPercentageUsedVal > 50 {
// 			colorFmt = color.Style{color.Magenta}
// 		} else if pvcPercentageUsedVal > 25 {
// 			colorFmt = color.Style{color.Yellow}
// 		}
//
// 		// var (
// 		// 	Reset  = "\033[0m"
// 		// 	Red    = "\033[31m"
// 		// 	Green  = "\033[32m"
// 		// 	Yellow = "\033[33m"
// 		// 	Blue   = "\033[34m"
// 		// 	Purple = "\033[35m"
// 		// 	Cyan   = "\033[36m"
// 		// 	Gray   = "\033[37m"
// 		// 	White  = "\033[97m"
// 		// )
// 		// if pvcPercentageUsedVal > 75 {
// 		// 	pvcPercentageUsedString = fmt.Sprintf("\033[31m%s\033[0m", pvcPercentageUsedString)
// 		// }
//
// 		thisRow := metav1.TableRow{Cells: []interface{}{
// 			fmt.Sprintf("%s", pvcRow.PVCName),
// 			fmt.Sprintf("%s", pvcRow.Namespace),
// 			fmt.Sprintf("%s", pvcRow.PodName),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.CapacityBytes)),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.UsedBytes)),
// 			fmt.Sprintf("%s", ConvertQuantityValueToHumanReadableIECString(pvcRow.AvailableBytes)),
// 			colorFmt.Sprintf("%.2f", pvcPercentageUsedVal),
// 			fmt.Sprintf("%d", pvcRow.InodesUsed),
// 			fmt.Sprintf("%d", pvcRow.InodesFree),
// 			fmt.Sprintf("%.2f", pvcRow.PercentageIUsed),
// 		}}
// 		rows = append(rows, thisRow)
// 	}
//
// 	table := &metav1.Table{
// 		ColumnDefinitions: columns,
// 		Rows:              rows,
// 	}
// 	out := bytes.NewBuffer([]byte{})
// 	printer := printers.NewTablePrinter(printers.PrintOptions{
// 		SortBy: "PVC",
// 	})
// 	printer.PrintObj(table, out)
// 	fmt.Printf("\n%s\n", out)
// }

// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
func ConvertQuantityValueToHumanReadableIECString(quantity *resource.Quantity) string {
	var val = quantity.Value()
	var suffix string

	TiConvertedVal := val / 1099511627776
	GiConvertedVal := val / 1073741824
	MiConvertedVal := val / 1048576
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

// func ConvertQuantityValueToHumanReadableIECStringFromSuffix(quantity *resource.Quantity, suffix string) string {
// 	var convertedValue = quantity.Value()
// 	switch suffix {
// 	case "Ki":
// 		// https://en.wikipedia.org/wiki/Kibibyte
// 		// 1 KiB = 2^10 bytes = 1024 bytes
// 		convertedValue = convertedValue / 1024
// 	case "Mi":
// 		// https://en.wikipedia.org/wiki/Mebibyte
// 		// 1 MiB = 2^20 bytes = 1048576 bytes = 1024 kibibytes
// 		convertedValue = convertedValue / 1048576
// 	case "Gi":
// 		// https://en.wikipedia.org/wiki/Gibibyte
// 		// 1 GiB = 2^30 bytes = 1073741824 bytes = 1024 mebibytes
// 		convertedValue = convertedValue / 1073741824
// 	case "Ti":
// 		// https://en.wikipedia.org/wiki/Tebibyte
// 		// 1 TiB = 2^40 bytes = 1099511627776 bytes = 1024 gibibytes
// 		convertedValue = convertedValue / 1099511627776
// 	default:
// 	}
// 	return fmt.Sprintf("%d%s", convertedValue, suffix)
// }

type OutputRowPVC struct {
	Namespace       string             `json:"namespace"`
	PVCName         string             `json:"pvcName"`
	PVName          string             `json:"pvName"`
	PodName         string             `json:"podName"`
	VolumeMountName string             `json:"volumeMountName"`
	AvailableBytes  *resource.Quantity `json:"availableBytes"` // TODO: use uint64 here as well? but resource.Quantity takes int64
	CapacityBytes   *resource.Quantity `json:"capacityBytes"`
	UsedBytes       *resource.Quantity `json:"usedBytes"`
	InodesFree      uint64             `json:"inodesFree"`
	Inodes          uint64             `json:"inodes"`
	InodesUsed      uint64             `json:"inodesUsed"`
	PercentageUsed  float64
	PercentageIUsed float64
}

type ServerResponseStruct struct {
	Pods []*Pod `json:"pods"`
}

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

	nodes, err := ListNodes(context.TODO(), clientset)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list nodes")
	}

	desiredNamespace := *flags.genericCliConfigFlags.Namespace
	// desiredNamespace := flags.namespace

	g, ctx := errgroup.WithContext(ctx)

	nodeChan := make(chan corev1.Node)
	outputRowPVCChan := make(chan *OutputRowPVC)

	nodeItems := nodes.Items
	g.Go(func() error {
		defer close(nodeChan)
		for _, node := range nodeItems {
			select {
			case nodeChan <- node:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	const numWorkers = 3
	for w := 1; w <= numWorkers; w++ {
		g.Go(func() error {
			return GetOutputRowPVCFromNodeChan(ctx, clientset, nodeChan, desiredNamespace, outputRowPVCChan)
		})
	}

	go func() {
		g.Wait()
		close(outputRowPVCChan)
	}()

	var sliceOfOutputRowPVC []*OutputRowPVC
	for outputRowPVC := range outputRowPVCChan {
		sliceOfOutputRowPVC = append(sliceOfOutputRowPVC, outputRowPVC)
	}
	return sliceOfOutputRowPVC, g.Wait()
}

func GetOutputRowPVCFromNodeChan(ctx context.Context, clientset *kubernetes.Clientset, nodeChan <-chan corev1.Node, desiredNamespace string, outputRowPVCChan chan<- *OutputRowPVC) error {
	for node := range nodeChan {
		log.Tracef("connecting to node: %s", node.Name)
		request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(node.Name).SubResource("proxy").Suffix("stats/summary")
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
				select {
				case <-ctx.Done():
					return ctx.Err()
				case outputRowPVCChan <- outputRowPVC:
					log.Debugf("Got metrics for pvc '%s' from node: '%s'", outputRowPVC.PVCName, node.Name)
				}
			}
		}
	}
	return nil
}

func GetOutputRowPVCFromPodAndVolume(ctx context.Context, clientset *kubernetes.Clientset, pod *Pod, vol *Volume, desiredNamespace string) *OutputRowPVC {
	var outputRowPVC *OutputRowPVC

	if 0 < len(desiredNamespace) {
		if vol.PvcRef.PvcNamespace != desiredNamespace {
			return nil
		} else {
			log.Debugf("restricting findings to namespace: '%s'", desiredNamespace)
		}
	}

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

func GetKubeConfigFromGenericCliConfigFlags(genericCliConfigFlags *genericclioptions.ConfigFlags) (*rest.Config, error) {
	config, err := genericCliConfigFlags.ToRESTConfig()
	return config, errors.Wrap(err, "failed to read kubeconfig")
}

func ListNodes(ctx context.Context, clientset *kubernetes.Clientset) (*corev1.NodeList, error) {
	log.Tracef("getting a list of all nodes")
	return clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
}

func GetPVNameFromPVCName(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pvcName string) (string, error) {
	var pvName string
	pvc, err := clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
	if err != nil {
		return pvName, err
	}
	pvName = pvc.Spec.VolumeName
	return pvName, err
}

func KubeConfigPath() (string, error) {
	log.Debugf("getting kubeconfig path based on user's home dir")
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrapf(err, "unable to get home dir")
	}
	return path.Join(home, ".kube", "config"), nil
}
