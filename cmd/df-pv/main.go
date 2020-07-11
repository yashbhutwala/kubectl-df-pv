package main

import (
	_ "k8s.io/client-go/plugin/pkg/client/auth" // required for auth, see: https://github.com/kubernetes/client-go/tree/v0.17.3/plugin/pkg/client/auth

	"github.com/yashbhutwala/kubectl-df-pv/pkg/df-pv"
)

func main() {
	df_pv.InitAndExecute()
}
