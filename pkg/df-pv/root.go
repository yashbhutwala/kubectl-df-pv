package df_pv

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
)

type flagpole struct {
	kubernetesConfigFlags *genericclioptions.ConfigFlags
	outputFormat          string
}

func setupRootCommand() *cobra.Command {
	flags := &flagpole{kubernetesConfigFlags: genericclioptions.NewConfigFlags(false)}
	var rootCmd = &cobra.Command{
		Use:   "df-pv",
		Short: "df-pv",
		Long:  `df-pv`,
		Args:  cobra.MaximumNArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			finishedCh := make(chan bool, 1)
			nodeName := make(chan string, 1)
			go func() {
				lastNodeName := ""
				for {
					select {
					case <-finishedCh:
						fmt.Printf("\r")
						return
					case n := <-nodeName:
						lastNodeName = n
					case <-time.After(time.Millisecond * 100):
						if lastNodeName == "" {
							fmt.Printf("\r  \033[36mSearching for PVCs\033[m")
						} else {
							fmt.Printf("\r  %s \n", lastNodeName)
						}
					}
				}
			}()
			defer func() {
				finishedCh <- true
			}()

			listOfRunningPvc, err := ListPVCs(flags, nodeName)
			if err != nil {
				return errors.Cause(err)
			}
			finishedCh <- true

			var columns = []metav1.TableColumnDefinition{
				{Name: "PVC", Type: "string"},
				{Name: "Namespace", Type: "string"},
				{Name: "Pod", Type: "string"},
				{Name: "Size", Type: "string"},
				{Name: "Used", Type: "string"},
				{Name: "Available", Type: "string"},
				{Name: "PercentUsed", Type: "string"},
				{Name: "iused", Type: "string"},
				{Name: "ifree", Type: "string"},
				{Name: "Percentiused", Type: "string"},
			}
			var rows []metav1.TableRow

			// use white as default
			// c := color.New(color.FgHiWhite)

			for _, pvc := range listOfRunningPvc {
				// if pvc.PercentageUsed > 75 || pvc.PercentageIUsed > 75 {
				//	c = color.New(color.FgHiRed)
				// } else if pvc.PercentageUsed > 50 || pvc.PercentageIUsed > 50 {
				//	c = color.New(color.FgHiMagenta)
				// } else if pvc.PercentageUsed > 25 || pvc.PercentageIUsed > 25 {
				//	c = color.New(color.FgHiYellow)
				// }
				thisRow := metav1.TableRow{Cells: []interface{}{
					fmt.Sprintf("%s", pvc.PVCName),
					fmt.Sprintf("%s", pvc.Namespace),
					fmt.Sprintf("%s", pvc.PodName),
					fmt.Sprintf("%s", convertQuantityValueToHumanReadableSIUnitString(pvc.CapacityBytes, flags.outputFormat)),
					fmt.Sprintf("%s", convertQuantityValueToHumanReadableSIUnitString(pvc.UsedBytes, flags.outputFormat)),
					fmt.Sprintf("%s", convertQuantityValueToHumanReadableSIUnitString(pvc.AvailableBytes, flags.outputFormat)),
					fmt.Sprintf("%.2f", pvc.PercentageUsed),
					fmt.Sprintf("%d", pvc.InodesUsed),
					fmt.Sprintf("%d", pvc.InodesFree),
					fmt.Sprintf("%.2f", pvc.PercentageIUsed),
				}}
				rows = append(rows, thisRow)
			}

			table := &metav1.Table{
				ColumnDefinitions: columns,
				Rows:              rows,
			}
			out := bytes.NewBuffer([]byte{})
			printer := printers.NewTablePrinter(printers.PrintOptions{
				SortBy: "PVC",
			})
			printer.PrintObj(table, out)
			fmt.Println(out.String())

			return nil
		},
	}
	rootCmd.Flags().StringVarP(&flags.outputFormat, "output", "o", "Gi", "output format for bytes; one of [Ki, Mi, Gi], see: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/#meaning-of-memory")

	flags.kubernetesConfigFlags.AddFlags(rootCmd.Flags())
	return rootCmd
}

// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
func convertQuantityValueToHumanReadableSIUnitString(quantity *resource.Quantity, suffix string) string {
	var convertedValue int64
	switch suffix {

	case "Mi":
		// https://en.wikipedia.org/wiki/Mebibyte
		// 1 MiB = 2^20 bytes = 1048576 bytes = 1024 kibibytes
		convertedValue = quantity.Value() / 1048576
	case "Gi":
		// https://en.wikipedia.org/wiki/Gibibyte
		// 1 GiB = 2^30 bytes = 1073741824 bytes = 1024 mebibytes
		convertedValue = quantity.Value() / 1073741824
	case "Ki":
		// https://en.wikipedia.org/wiki/Kibibyte
		// 1 KiB = 2^10 bytes = 1024 bytes
		convertedValue = quantity.Value() / 1024
	}
	return fmt.Sprintf("%d%s", convertedValue, suffix)
}

func InitAndExecute() {
	rootCmd := setupRootCommand()
	if err := errors.Wrapf(rootCmd.Execute(), "run df-pv root command"); err != nil {
		log.Fatalf("unable to run root command: %+v", err)
		os.Exit(1)
	}
}

type OutputPVC struct {
	PodName   string `json:"podName"`
	Namespace string `json:"namespace"`

	PVCName string `json:"pvcName"`

	AvailableBytes *resource.Quantity `json:"availableBytes"`
	CapacityBytes  *resource.Quantity `json:"capacityBytes"`
	UsedBytes      *resource.Quantity `json:"usedBytes"`
	PercentageUsed float64

	InodesFree      int64 `json:"inodesFree"`
	Inodes          int64 `json:"inodes"`
	InodesUsed      int64 `json:"inodesUsed"`
	PercentageIUsed float64

	VolumeMountName string `json:"volumeMountName"`
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
	UsedBytes int64 `json:"usedBytes"`

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
	InodesUsed int64 `json:"inodesUsed"`

	// Inodes represents the total number of inodes available in the volume.
	// For volumes that share a filesystem with the host (e.g. emptydir, hostpath),
	// this is the inodes available in the underlying storage,
	// and will not equal InodesUsed + InodesFree as the fs is shared.
	Inodes int64 `json:"inodes"`

	// InodesFree represent the inodes available for the volume.  For Volumes that share
	// a filesystem with the host (e.g. emptydir, hostpath), this is the free inodes
	// on the underlying storage, and is shared with host processes and other volumes
	InodesFree int64 `json:"inodesFree"`

	Name   string `json:"name"`
	PvcRef struct {
		PvcName      string `json:"name"`
		PvcNamespace string `json:"namespace"`
	} `json:"pvcRef"`
}

func ListPVCs(flags *flagpole, outputCh chan string) ([]*OutputPVC, error) {
	config, err := flags.kubernetesConfigFlags.ToRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read kubeconfig")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create clientset")
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list nodes")
	}

	var listOfPvc []*OutputPVC
	var jsonConvertedIntoStruct ServerResponseStruct

	for _, node := range nodes.Items {
		request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(node.Name).SubResource("proxy").Suffix("stats/summary")
		responseRawArrayOfBytes, err := request.DoRaw(context.Background())
		if err != nil {
			return nil, errors.Wrap(err, "failed to get stats from node")
		}

		err = json.Unmarshal(responseRawArrayOfBytes, &jsonConvertedIntoStruct)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert the response from server")
		}

		for _, pod := range jsonConvertedIntoStruct.Pods {
			for _, vol := range pod.ListOfVolumes {
				desiredNamespace := *flags.kubernetesConfigFlags.Namespace
				if 0 < len(desiredNamespace){
					if vol.PvcRef.PvcNamespace != desiredNamespace {
						continue
					}
				}

				if 0 < len(vol.PvcRef.PvcName) {
					runningPvc := &OutputPVC{
						PodName:   pod.PodRef.Name,
						Namespace: pod.PodRef.Namespace,

						PVCName:        vol.PvcRef.PvcName,
						AvailableBytes: resource.NewQuantity(vol.AvailableBytes, resource.BinarySI),
						CapacityBytes:  resource.NewQuantity(vol.CapacityBytes, resource.BinarySI),
						UsedBytes:      resource.NewQuantity(vol.UsedBytes, resource.BinarySI),
						PercentageUsed: (float64(vol.UsedBytes) / float64(vol.CapacityBytes)) * 100.0,

						Inodes:          vol.Inodes,
						InodesFree:      vol.InodesFree,
						InodesUsed:      vol.InodesUsed,
						PercentageIUsed: (float64(vol.InodesUsed) / float64(vol.Inodes)) * 100.0,

						VolumeMountName: vol.Name,
					}
					outputCh <- fmt.Sprintf("Got metrics for pvc '%s' from node: '%s'", runningPvc.PVCName, node.Name)
					listOfPvc = append(listOfPvc, runningPvc)
				}
			}
		}

		// clear out the object for reuse
		jsonConvertedIntoStruct = ServerResponseStruct{}
	}

	return listOfPvc, nil
}
