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
## Install
1. Install package
```
go install github.com/michaelvl/go-aws-eks-get-token@latest
```

2. Update kubeconfig to switch to `go-aws-eks-get-token` (NOTE: The following command will do it for all `users`in your `~/.kube/config`
```
yq eval -i  '(.users[] | select(.user.exec.command == "aws") | .user.exec.command) = "go-aws-eks-get-token"' ~/.kube/config
```

3. Set `AWS_PROFILE=<profile name>` for all users (that uses this tool) in `~/kube/.config`
Fx.
```
- name: <your user's name>
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      ...
      command: go-aws-eks-get-token
      env:
        - name: AWS_PROFILE
          value: <your AWS profilename>
```


## Usage

This tool is generally a drop-in replacement for the `aws` CLI in `kubectl`
files when used with the following arguments:

```shell
eks get-token --cluster-name <CLUSTER> --output json
```
