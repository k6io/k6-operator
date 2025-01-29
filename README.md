 ![data flow](assets/data-flow.png)

# k6 Operator

`grafana/k6-operator` is a Kubernetes operator for running distributed [k6](https://github.com/grafana/k6) tests in your cluster. k6 Operator introduces two CRDs:

- `TestRun` CRD
- `PrivateLoadZone` CRD

The `TestRun` CRD is a representation of a single k6 test executed once. `TestRun` supports various configuration options that allow you to adapt to different Kubernetes setups. You can find a description of the more common options [here](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/usage/common-options/), and the full list of options can be found in the [definition itself](https://github.com/grafana/k6-operator/blob/main/config/crd/bases/k6.io_testruns.yaml).

The `PrivateLoadZone` CRD is a representation of a [load zone](https://grafana.com/docs/grafana-cloud/testing/k6/author-run/use-load-zones/), which is a k6 term for a set of nodes within a cluster designated to execute k6 test runs. `PrivateLoadZone` is integrated with [Grafana Cloud k6](https://grafana.com/products/cloud/k6/) and requires a [Grafana Cloud account](https://grafana.com/auth/sign-up/create-user). You can find a guide describing how to set up a `PrivateLoadZone` [here](https://grafana.com/docs/grafana-cloud/testing/k6/author-run/private-load-zone-v2/), while billing details can be found [here](https://grafana.com/docs/grafana-cloud/cost-management-and-billing/understand-your-invoice/k6-invoice/).

## Documentation

You can find the latest k6 Operator documentation in the [Grafana k6 OSS docs](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/usage/common-options/).

For additional resources:

- :book: A guide [Running distributed load tests on Kubernetes](https://grafana.com/blog/2022/06/23/running-distributed-load-tests-on-kubernetes/).
- :book: A guide [Running distributed tests](https://grafana.com/docs/k6/latest/testing-guides/running-distributed-tests/).
- :movie_camera: Grafana Office Hours [Load Testing on Kubernetes with k6 Private Load Zones](https://www.youtube.com/watch?v=RXLavQT58YA).

Common samples are available in the `config/samples` and `e2e/` folders in this repo, both for the `TestRun` and `PrivateLoadZone` CRDs.

## Contributing

### Requests and feedback

We are always interested in your feedback! If you encounter problems during the k6 Operator usage, check out the [troubleshooting guide](https://grafana.com/docs/k6/latest/set-up/set-up-distributed-k6/troubleshooting/). If you have questions on how to use the k6 Operator, you can post them on the [Grafana community forum](https://community.grafana.com/c/grafana-k6/k6-operator/73).

For new feature requests and bug reports, consider opening an issue in this repository. First, check the [existing issues](https://github.com/grafana/k6-operator/issues) in case a similar report already exists. If it does, add a comment about your use case or upvote it.

For bug reports, please use [this template](https://github.com/grafana/k6-operator/issues/new?assignees=&labels=bug&projects=&template=bug.yaml). If you think there is a missing feature, please use [this template](https://github.com/grafana/k6-operator/issues/new?assignees=&labels=enhancement&projects=&template=feat_req.yaml).

### Development

<!-- TODO: pull out into contributing guide -->

When submitting a PR, it's preferable to work on an open issue. If an issue does not exist, create it. An issue allows us to validate the problem, gather additional feedback from the community, and avoid unnecessary work.

<!-- 
Some GitHub issues have a ["good first issue" label](https://github.com/grafana/k6-operator/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22). These are the issues that should be good for newcomers. The issues with the ["help wanted" label](https://github.com/grafana/k6-operator/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22) are the ones that could use some community help or additional user feedback. -->

There are many options for setting up a local Kubernetes cluster for development, and any of them can be used for local development of the k6 Operator. One option is to create a [kind cluster](https://kind.sigs.k8s.io/docs/user/quick-start/).

Additionally, you'll need to install the following tooling:

- [Golang](https://go.dev/doc/install)
- [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/)
- [operator-sdk](https://sdk.operatorframework.io/docs/installation/): optional as most common changes can be done without it

To execute unit tests, use these commands:

```bash
make test
```

To execute e2e test locally:

- `make e2e` for kustomize and `make e2e-helm` for Helm
- validate tests have been run
- `make e2e-cleanup`
