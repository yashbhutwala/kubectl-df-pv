.PHONY: test
test:
	go test ./pkg/... ./cmd/... -coverprofile cover.out

.PHONY: bin
bin: fmt vet
	go build -o bin/df-pv github.com/yashbhutwala/kubectl-df-pv/cmd/plugin

.PHONY: fmt
fmt:
	go fmt ./pkg/... ./cmd/...

.PHONY: vet
vet:
	go vet ./pkg/... ./cmd/...

.PHONY: release
# release: export GITHUB_TOKEN = $(shell echo ${GITHUB_TOKEN})
#release:
#	curl -sL https://git.io/goreleaser | bash -s -- --rm-dist --config deploy/.goreleaser.yml
