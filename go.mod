module github.com/yashbhutwala/kubectl-df-pv

go 1.15

require (
	github.com/go-openapi/strfmt v0.19.5 // indirect
	github.com/jedib0t/go-pretty v4.3.0+incompatible
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/cobra v1.0.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/cli-runtime v0.19.2
	k8s.io/client-go v11.0.0+incompatible
)

replace k8s.io/client-go v11.0.0+incompatible => k8s.io/client-go v0.19.2
