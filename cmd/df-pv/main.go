package main

import (
	_ "k8s.io/client-go/plugin/pkg/client/auth" // required for auth, see: https://github.com/kubernetes/client-go/tree/v0.17.3/plugin/pkg/client/auth

	df_pv "github.com/X-dark/kubectl-df-pv/pkg/df-pv"
)

func main() {
	df_pv.InitAndExecute()
}
