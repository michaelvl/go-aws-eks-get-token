# go-aws-eks-get-token

This repository provides a Go replacement for `aws eks get-token`, generating
EKS exec tokens suitable for use with `kubectl`. It features cached credentials,
maximum expiry, and is intended for CLI and automation use.

```shell
# standard aws cli
time kubectl get pods
... 0.37s user 0.20s system 24% cpu 2.320 total

# this tools using cached token
time kubectl get pods
... 0.08s user 0.05s system 52% cpu 0.335 total
```

## Usage

This tool is generally a drop-in replacement for the `aws` CLI in `kubectl`
files when used with the following arguments:

```shell
eks get-token --cluster-name <CLUSTER> --output json
```
