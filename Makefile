# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/hashanhirantha/auto-tls-certreloader:latest

# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded.
ENVTEST_K8S_VERSION = 1.31.0

# Tool binaries
CONTROLLER_GEN ?= $(shell which controller-gen 2>/dev/null)
ENVTEST ?= $(shell which setup-envtest 2>/dev/null)

.PHONY: all
all: generate fmt vet build

##@ General

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: generate
generate: ## Generate code (DeepCopy methods).
	controller-gen object paths="./api/..."

.PHONY: manifests
manifests: ## Generate CRD manifests and RBAC.
	controller-gen crd rbac:roleName=certreloader-role paths="./..." output:crd:dir=config/crd/bases

.PHONY: fmt
fmt: ## Run go fmt.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet.
	go vet ./...

.PHONY: test
test: ## Run unit tests.
	go test ./internal/... -coverprofile cover.out

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests (requires a running cluster).
	go test ./test/e2e/... -v -timeout 10m

##@ Build

.PHONY: build
build: generate fmt vet ## Build the operator binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: generate fmt vet ## Run the operator locally against the configured cluster.
	go run cmd/main.go

.PHONY: docker-build
docker-build: ## Build the Docker image.
	docker build -t $(IMG) .

.PHONY: docker-push
docker-push: ## Push the Docker image.
	docker push $(IMG)

##@ Deployment

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster (uses current kubeconfig).
	kubectl apply -f config/crd/bases/

.PHONY: uninstall
uninstall: ## Uninstall CRDs from the K8s cluster.
	kubectl delete -f config/crd/bases/

.PHONY: deploy
deploy: manifests ## Deploy controller to the K8s cluster.
	kubectl apply -k config/default/

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster.
	kubectl delete -k config/default/

.PHONY: kind-cluster
kind-cluster: ## Create a local kind cluster for development.
	kind create cluster --name certreloader --config hack/kind-config.yaml

.PHONY: kind-load
kind-load: docker-build ## Load the Docker image into the kind cluster.
	kind load docker-image $(IMG) --name certreloader

##@ Tool Dependencies

.PHONY: controller-gen
controller-gen: ## Install controller-gen if not present.
	@which controller-gen || go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

.PHONY: envtest
envtest: ## Install setup-envtest if not present.
	@which setup-envtest || go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
