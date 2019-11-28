package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	// third party dependencies
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tj/go-spin"
	//"github.com/fatih/color"

	// k8s dependencies
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"

	// this repo
	"github.com/yashbhutwala/kubectl-df-pv/pkg/plugin"
)

var (
	KubernetesConfigFlags *genericclioptions.ConfigFlags
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "df-pv",
		Short:         "",
		Long:          `.`,
		SilenceErrors: true,
		SilenceUsage:  true,
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			df := plugin.DisplayFree{}

			s := spin.New()
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
							fmt.Printf("\r  \033[36mSearching for PVCs\033[m %s", s.Next())
						} else {
							fmt.Printf("\r  \033[36mSearching for PVCs\033[m %s (%s)", s.Next(), lastNodeName)
						}
					}
				}
			}()
			defer func() {
				finishedCh <- true
			}()

			listOfRunningPvc, err := df.ListPVCs(KubernetesConfigFlags, nodeName)
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
			//c := color.New(color.FgHiWhite)

			for _, pvc := range listOfRunningPvc {

				//if pvc.PercentageUsed > 75 || pvc.PercentageIUsed > 75 {
				//	c = color.New(color.FgHiRed)
				//} else if pvc.PercentageUsed > 50 || pvc.PercentageIUsed > 50 {
				//	c = color.New(color.FgHiMagenta)
				//} else if pvc.PercentageUsed > 25 || pvc.PercentageIUsed > 25 {
				//	c = color.New(color.FgHiYellow)
				//}

				//thisRow := metav1.TableRow{Cells: []interface{}{
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
				//}}
				thisRow := metav1.TableRow{Cells: []interface{}{
					fmt.Sprintf("%s", pvc.PvcName),
					fmt.Sprintf("%s", pvc.Namespace),
					fmt.Sprintf("%s", pvc.PodName),
					fmt.Sprintf("%s", pvc.CapacityBytes.String()),
					fmt.Sprintf("%s", pvc.UsedBytes.String()),
					fmt.Sprintf("%s", pvc.AvailableBytes.String()),
					fmt.Sprintf("%.2f", pvc.PercentageUsed),
					fmt.Sprintf("%d", pvc.INodesUsed),
					fmt.Sprintf("%d", pvc.INodesFree),
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

	cobra.OnInitialize(initConfig)

	KubernetesConfigFlags = genericclioptions.NewConfigFlags(false)
	KubernetesConfigFlags.AddFlags(cmd.Flags())

	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.AutomaticEnv()
}
