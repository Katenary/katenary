## End-to-end tests on a local kind cluster

# Name of the kind cluster used by the end-to-end tests.
E2E_CLUSTER ?= katenary-e2e

.PHONY: e2e e2e-clean

# Run the end-to-end scenarios. The cluster is kept between runs so that
# successive runs are fast; use e2e-clean to remove it.
e2e:
	./e2e/run.sh

# Delete the kind cluster and its kubeconfig context.
e2e-clean:
	kind delete cluster --name $(E2E_CLUSTER)
