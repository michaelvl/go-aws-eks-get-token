package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// ExecCredential is the format for kubectl ExecCredential
type ExecCredential struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Status     struct {
		ExpirationTimestamp string `json:"expirationTimestamp"`
		Token               string `json:"token"`
	} `json:"status"`
}

const (
	// AWS EKS maximum token duration is 15 minutes (900 seconds)
	maxTokenDuration   = 900 * time.Second
	cacheExpiryPadding = 30 * time.Second
)

func main() {
	region := flag.String("region", "", "AWS region (required)")
	flag.Parse()
	if *region == "" {
		flag.Usage()
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) < 2 || args[0] != "eks" || args[1] != "get-token" {
		fmt.Fprintln(os.Stderr, "expected 'eks get-token' subcommand(s)")
		os.Exit(1)
	}

	flag.NewFlagSet("eks", flag.ExitOnError)
	getTokenCmd := flag.NewFlagSet("get-token", flag.ExitOnError)
	cluster := getTokenCmd.String("cluster-name", "", "EKS cluster name (required)")
	output := getTokenCmd.String("output", "json", "Output format (must be 'json')")
	err := getTokenCmd.Parse(args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing arguments: %s\n", err)
		os.Exit(1)
	}

	if *output != "json" {
		fmt.Fprintln(os.Stderr, "only 'json' is accepted for --output")
		os.Exit(1)
	}
	if *cluster == "" {
		getTokenCmd.Usage()
		os.Exit(1)
	}

	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		fmt.Fprintln(os.Stderr, "AWS_PROFILE environment variable is required")
		os.Exit(1)
	}

	cachePath, err := kubeCacheFilePath(profile, *cluster)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get cache path: %v\n", err)
		os.Exit(1)
	}

	// Try to use cached token if valid
	if cred, ok := tryReadValidCache(cachePath); ok {
		fmt.Println(string(cred))
		return
	}

	// Create context for AWS operations
	ctx := context.Background()

	// Set up AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(*region),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load AWS config: %v\n", err)
		os.Exit(1)
	}

	// Create STS client and presign client
	stsSvc := sts.NewFromConfig(cfg)

	// Create custom presigner with expiration support
	presigner := v4.NewSigner()
	customPresigner := &eksPresigner{
		signer:  presigner,
		expires: maxTokenDuration,
	}

	presignClient := sts.NewPresignClient(stsSvc, func(po *sts.PresignOptions) {
		po.Presigner = customPresigner
	})

	// Presign the GetCallerIdentity request with custom header
	presignResult, err := presignClient.PresignGetCallerIdentity(ctx,
		&sts.GetCallerIdentityInput{},
		func(po *sts.PresignOptions) {
			po.ClientOptions = append(po.ClientOptions, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
					return stack.Build.Add(middleware.BuildMiddlewareFunc(
						"AddEKSHeader",
						func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (
							middleware.BuildOutput, middleware.Metadata, error,
						) {
							if req, ok := in.Request.(*smithyhttp.Request); ok {
								req.Header.Add("x-k8s-aws-id", *cluster)
							}
							return next.HandleBuild(ctx, in)
						},
					), middleware.Before)
				})
			})
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to presign STS request: %v\n", err)
		os.Exit(1)
	}

	urlStr := presignResult.URL

	token := "k8s-aws-v1." + encodeBase64Url(urlStr)
	expiry := time.Now().Add(maxTokenDuration).UTC().Format(time.RFC3339)

	cred := ExecCredential{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Kind:       "ExecCredential",
	}
	cred.Status.ExpirationTimestamp = expiry
	cred.Status.Token = token

	// Marshal to JSON
	out, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal ExecCredential: %v\n", err)
		os.Exit(1)
	}

	// Write to disk
	if err := os.WriteFile(cachePath, out, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write ExecCredential to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

// tryReadValidCache checks for a cached credential file, and returns its contents if it's still valid (expiry > 30s).
func tryReadValidCache(path string) ([]byte, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var cred ExecCredential
	if err := json.Unmarshal(data, &cred); err != nil {
		return nil, false
	}
	expiry, err := time.Parse(time.RFC3339, cred.Status.ExpirationTimestamp)
	if err != nil {
		return nil, false
	}
	if time.Until(expiry) > cacheExpiryPadding {
		return data, true
	}
	return nil, false
}

// encodeBase64Url encodes a string to URL-safe base64 with no padding, per EKS requirements
func encodeBase64Url(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

// kubeCacheFilePath returns the file path for the cached token, ensuring .kube and .kube/cache exist.
func kubeCacheFilePath(profile, cluster string) (string, error) {
	usr, err := user.Current()
	if err != nil {
		return "", err
	}
	kubeDir := filepath.Join(usr.HomeDir, ".kube")
	cacheDir := filepath.Join(kubeDir, "cache")

	// Ensure .kube/cache directory exists
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create .kube/cache directory: %w", err)
	}

	filename := fmt.Sprintf("eks-token-%s-%s.json", profile, cluster)
	return filepath.Join(cacheDir, filename), nil
}

// eksPresigner is a custom presigner that adds X-Amz-Expires query parameter for EKS tokens
type eksPresigner struct {
	signer  *v4.Signer
	expires time.Duration
}

// PresignHTTP implements the HTTPPresignerV4 interface with custom expiration
func (p *eksPresigner) PresignHTTP(
	ctx context.Context,
	credentials aws.Credentials,
	r *http.Request,
	payloadHash string,
	service string,
	region string,
	signingTime time.Time,
	optFns ...func(*v4.SignerOptions),
) (string, http.Header, error) {
	// Add X-Amz-Expires query parameter before signing
	q := r.URL.Query()
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", int(p.expires.Seconds())))
	r.URL.RawQuery = q.Encode()

	return p.signer.PresignHTTP(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}
