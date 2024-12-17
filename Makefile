# Current Operator version
VERSION ?= 0.0.18
# Default bundle image tag
BUNDLE_IMG ?= k6-controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

CONTROLLER_GEN_VERSION=v0.16.1
CONTROLLER_GEN=$(GOBIN)/controller-gen

# Image to use for building Go
GO_BUILDER_IMG ?= "golang:1.22"
# Image URL to use all building/pushing image targets
IMG_NAME ?= ghcr.io/grafana/k6-operator
IMG_TAG ?= latest
# Default dockerfile to build
DOCKERFILE ?= "Dockerfile.controller"
CRD_OPTIONS ?= "crd" #:maxDescLen=0"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_VERSION ?= latest # ref. https://github.com/kubernetes-sigs/controller-runtime/tree/main/tools/setup-envtest
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
ENVTEST_K8S_VERSION ?= 1.30.0
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
KUBEBUILDER_ASSETS_ROOT=/tmp
KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS_ROOT)/kubebuilder/bin

test: generate fmt vet manifests
	export KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS); setup-envtest use --use-env -p env $(ENVTEST_K8S_VERSION); go test ./... -coverprofile cover.out

test-setup:
	curl -L -O "https://storage.googleapis.com/kubebuilder-tools/kubebuilder-tools-$(ENVTEST_K8S_VERSION)-$(GOOS)-$(GOARCH).tar.gz"
	tar -zxvf kubebuilder-tools-$(ENVTEST_K8S_VERSION)-$(GOOS)-$(GOARCH).tar.gz
	mv kubebuilder $(KUBEBUILDER_ASSETS_ROOT)
	export KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS); go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(ENVTEST_VERSION)

e2e: deploy
	kubectl create configmap crocodile-stress-test --from-file e2e/test.js
	kubectl apply -f e2e/test.yaml

e2e-cleanup:
	kubectl delete configmap crocodile-stress-test
	kubectl delete -f e2e/test.yaml

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_NAME}:${IMG_TAG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Delete operator from a cluster
delete: manifests kustomize
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Run golangci-lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.61.0
	golangci-lint --timeout 5m run ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG_NAME}:${IMG_TAG} -f ${DOCKERFILE} --build-arg GO_BUILDER_IMG=${GO_BUILDER_IMG}

# Push the docker image
docker-push:
	docker push ${IMG_NAME}:${IMG_TAG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
	@{ \
	if ! which $(CONTROLLER_GEN) || [ 'Version $(CONTROLLER_GEN_VERSION)' != "$$($(CONTROLLER_GEN) --version)" ]; then\
		set -e ;\
		go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION) ;\
	fi;\
	}

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go install sigs.k8s.io/kustomize/kustomize/v4@v4.5.5 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG_NAME}:${IMG_TAG}
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

generate-crd-docs:
	docker run -u $(id -u):$(id -g) --rm -v ${PWD}:/workdir ghcr.io/fybrik/crdoc:latest --resources /workdir/config/crd/bases --output /workdir/docs/crd-generated.md --template /workdir/docs/crd.tmpl

# ===============================================================
# This section is only about the HELM deployment of the operator
# ===============================================================

e2e-helm: deploy-helm
	kubectl create configmap crocodile-stress-test --from-file e2e/test.js
	kubectl apply -f e2e/test.yaml

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy-helm: manifests helm
	$(HELM) upgrade --install --wait k6-operator ./charts/k6-operator -f ./charts/k6-operator/values.yaml --set manager.image.name=$(IMG_NAME) --set manager.image.tag=$(IMG_TAG)

helm-template: manifests helm
	$(HELM) template k6-operator ./charts/k6-operator -f ./charts/k6-operator/values.yaml --set manager.image.name=$(IMG_NAME) --set manager.image.tag=$(IMG_TAG)

helm-docs:
	go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2
	$(shell go env GOPATH)/bin/helm-docs

helm-schema:
	go install github.com/dadav/helm-schema/cmd/helm-schema@0.12.0
	$(shell go env GOPATH)/bin/helm-schema --chart-search-root ./charts/k6-operator

# Delete operator from a cluster
delete-helm: manifests helm
	$(HELM) uninstall k6-operator

helm:
ifeq (, $(shell which helm))
	@{ \
	set -e ;\
	HELM_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$HELM_GEN_TMP_DIR ;\
	curl -fsSL -o get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
	chmod 700 get_helm.sh ;\
	./get_helm.sh ;\
	rm -rf $$HELM_GEN_TMP_DIR ;\
	}
HELM=$(shell which helm)
else
HELM=$(shell which helm)
endif
