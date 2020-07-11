package plugin

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

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
	Time           metav1.Time            `json:"time"`
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

func (df DisplayFree) ListPVCs(configFlags *genericclioptions.ConfigFlags, outputCh chan string) ([]RunningPvc, error) {
	config, err := configFlags.ToRESTConfig()
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

		// outputCh <- fmt.Sprintf("Node: %s/", node.Name)

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
