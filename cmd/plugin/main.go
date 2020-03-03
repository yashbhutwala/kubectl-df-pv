package main

import (
	"github.com/yashbhutwala/kubectl-df-pv/cmd/plugin/cli"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // required for auth, see: https://github.com/kubernetes/client-go/tree/v0.17.3/plugin/pkg/client/auth
)

func main() {
	cli.InitAndExecute()
}
