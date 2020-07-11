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
			df := DisplayFree{}
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
							fmt.Printf("\r  \033[36mSearching for PVCs\033[m ")
						} else {
							fmt.Printf("\r  \033[36mSearching for PVCs\033[m (%s)", lastNodeName)
						}
					}
				}
			}()
			defer func() {
				finishedCh <- true
			}()

			listOfRunningPvc, err := df.ListPVCs(flags, nodeName)
			if err != nil {
				return errors.Cause(err)
			}
			finishedCh <- true

			columns := []metav1.TableColumnDefinition{
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

				// thisRow := metav1.TableRow{Cells: []interface{}{
				//	c.Sprintf("%s", pvc.PvcName),
				//	c.Sprintf("%s", pvc.Namespace),
				//	c.Sprintf("%s", pvc.PodName),
				//	c.Sprintf("%s", pvc.CapacityBytes.String()),
				//	c.Sprintf("%s", pvc.UsedBytes.String()),
				//	c.Sprintf("%s", pvc.AvailableBytes.String()),
				//	c.Sprintf("%.2f", pvc.PercentageUsed),
				//	c.Sprintf("%d", pvc.INodesUsed),
				//	c.Sprintf("%d", pvc.INodesFree),
				//	c.Sprintf("%.2f", pvc.PercentageIUsed),
				// }}
				thisRow := metav1.TableRow{Cells: []interface{}{
					fmt.Sprintf("%s", pvc.PvcName),
					fmt.Sprintf("%s", pvc.Namespace),
					fmt.Sprintf("%s", pvc.PodName),
					fmt.Sprintf("%s", pvc.CapacityBytes.String()),
					fmt.Sprintf("%s", pvc.UsedBytes.String()),
					fmt.Sprintf("%s", pvc.AvailableBytes.String()),
					fmt.Sprintf("%.2f", pvc.PercentageUsed),
					fmt.Sprintf("%s", pvc.INodesUsed.String()),
					fmt.Sprintf("%s", pvc.INodesFree.String()),
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
	rootCmd.Flags().StringVarP(&flags.outputFormat, "output", "o", "h", "output format")

	// KubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
	// KubernetesConfigFlags.AddFlags(rootCmd.Flags())
	return rootCmd
}

func InitAndExecute() {
	rootCmd := setupRootCommand()
	if err := errors.Wrapf(rootCmd.Execute(), "run df-pv root command"); err != nil {
		log.Fatalf("unable to run root command: %+v", err)
		os.Exit(1)
	}
}

type DisplayFree struct {
}

type RunningPvc struct {
	PodName   string `json:"podName"`
	Namespace string `json:"namespace"`

	PvcName string `json:"pvcName"`

	AvailableBytes resource.Quantity `json:"availableBytes"`
	CapacityBytes  resource.Quantity `json:"capacityBytes"`
	UsedBytes      resource.Quantity `json:"usedBytes"`
	PercentageUsed float64

	INodesFree      resource.Quantity `json:"inodesFree"`
	INodes          resource.Quantity `json:"inodes"`
	INodesUsed      resource.Quantity `json:"inodesUsed"`
	PercentageIUsed float64

	VolumeMountName string `json:"volumeMountName"`
}

type ServerResponseStruct struct {
	Pods []Pod `json:"pods"`
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
	ListOfVolumes []Volume `json:"volume"`
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
	Time           metav1.Time       `json:"time"`
	AvailableBytes resource.Quantity `json:"availableBytes"`
	CapacityBytes  resource.Quantity `json:"capacityBytes"`
	UsedBytes      resource.Quantity `json:"usedBytes"`
	INodesFree     resource.Quantity `json:"inodesFree"`
	INodes         resource.Quantity `json:"inodes"`
	INodesUsed     resource.Quantity `json:"inodesUsed"`
	Name           string            `json:"name"`
	PvcRef         struct {
		PvcName      string `json:"name"`
		PvcNamespace string `json:"namespace"`
	} `json:"pvcRef"`
}

func (df DisplayFree) ListPVCs(flags *flagpole, outputCh chan string) ([]RunningPvc, error) {
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

	var listOfPvc []RunningPvc
	var jsonConvertedIntoStruct ServerResponseStruct

	for _, node := range nodes.Items {

		outputCh <- fmt.Sprintf("Node: %s/", node.Name)

		request := clientset.CoreV1().RESTClient().Get().Resource("nodes").Name(node.Name).SubResource("proxy").Suffix("stats/summary")
		responseRawArrayOfBytes, err := request.DoRaw(context.TODO())
		if err != nil {
			return nil, errors.Wrap(err, "failed to get stats from node")
		}

		err = json.Unmarshal(responseRawArrayOfBytes, &jsonConvertedIntoStruct)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert the response from server")
		}

		for _, pod := range jsonConvertedIntoStruct.Pods {
			for _, vol := range pod.ListOfVolumes {
				if len(vol.PvcRef.PvcName) != 0 {
					runningPvc := RunningPvc{
						PodName:   pod.PodRef.Name,
						Namespace: pod.PodRef.Namespace,

						PvcName:        vol.PvcRef.PvcName,
						AvailableBytes: vol.AvailableBytes,
						CapacityBytes:  vol.CapacityBytes,
						UsedBytes:      vol.UsedBytes,
						PercentageUsed: (float64(vol.UsedBytes.Value()) / float64(vol.CapacityBytes.Value())) * 100.0,

						INodes:          vol.INodes,
						INodesFree:      vol.INodesFree,
						INodesUsed:      vol.INodesUsed,
						PercentageIUsed: (float64(vol.INodesUsed.Value()) / float64(vol.INodes.Value())) * 100.0,

						VolumeMountName: vol.Name,
					}

					// outputCh <- fmt.Sprintf("%s/%s", node.Name, runningPvc.PvcName)
					listOfPvc = append(listOfPvc, runningPvc)
				}
			}
		}

		// clear out the object for reuse
		jsonConvertedIntoStruct = ServerResponseStruct{}
	}

	return listOfPvc, nil
}
