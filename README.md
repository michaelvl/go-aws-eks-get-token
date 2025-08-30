# go-aws-eks-get-token

This repository provides a Go replacement for `aws eks get-token`, generating EKS exec tokens suitable for use with `kubectl`. It features cached credentials, maximum expiry, and is intended for CLI and automation use.

---

## 📝 Chat History

> **User:**  
> I would like you to write a golang replacement for using the aws cli eks get-token action. The program should accept a cluster name and aws region as arguments using the flag package. Also, aws auth should be through the environment variable AWS_PROFILE. The program should fetch a token and write it to a local file in the ExecCredentials format understood by kubectl. The file should be stored in the directory .kube/cache with a filename including the cluster name. Also the ExecCredentials should be output to stdout  
> **Copilot:**  
> (Provided Go code for the utility)  
>  
> **User:**  
> I would like to extend the expiry time to the maximum allowed by aws and the program, before requesting a new token, should read a possible previously saved version and if the expiry time of this cached version is more than 30 seconds into the future, then do not request a new token and output the cached version instead.  
> **Copilot:**  
> (Provided updated Go code with cache and max expiry features)  
>  
> **User:**  
> When I run this for the first time, the function kubeCacheFilePath will fail and I will never be able to create an initial version of the ExecCredentials. Please correct that  
> **Copilot:**  
> (Provided a fix to kubeCacheFilePath for fallback to $HOME)  
>  
> **User:**  
> discard this change  
> **Copilot:**  
> Understood, I will discard the previous proposal for the kubeCacheFilePath change.  
>  
> **User:**  
> go back to the previous version, version 2  
> **Copilot:**  
> (Reposted the code from version 2)  
>  
> **User:**  
> extend kubecachefilepath to create either .kube and cache folders if they do not exist  
> **Copilot:**  
> (Provided an updated kubeCacheFilePath that ensures directories exist)  
>  
> **User:**  
> include this change in the previous version  
> **Copilot:**  
> (Integrated the change in the full code)  
>  
> **User:**  
> please create a private repository called go-aws-eks-get-token and commit this code together with a go.mod file, a makefile for local building and linting using golangci-lint, a goreleaser config file and github action to build and release the code. Also include a nix flake file. Our full chat about this should go into the README  
> **Copilot:**  
> (Informed user to create the repo manually; readiness for next steps)  
>  
> **User:**  
> I have created the repository. Please do as previously asked. Also, include an apache 2 license  

---

## Features

- Acts as a `aws eks get-token` CLI replacement
- Uses Go, no Python/JQ/CLI dependencies
- Respects `AWS_PROFILE` for authentication
- Accepts `-cluster` and `-region` flags
- Caches tokens in `~/.kube/cache/eks-token-<cluster>.json`
- Outputs exec credentials to both file and stdout
- Only regenerates token if cache is expired (max AWS expiry minus 30s)
- Includes make targets, CI, GoReleaser config, and Nix flake for reproducibility

---

## License

Apache License 2.0, see [LICENSE](LICENSE).

---

## Quickstart

```sh
make build
AWS_PROFILE=yourprofile ./go-aws-eks-get-token -cluster=my-cluster -region=eu-west-1
```
