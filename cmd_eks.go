package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
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
	"github.com/spf13/cobra"
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

var (
	eksRegion      string
	clusterName    string
	outputFormat   string
)

var eksCmd = &cobra.Command{
	Use:   "eks",
	Short: "EKS operations",
}

var getTokenCmd = &cobra.Command{
	Use:   "get-token",
	Short: "Get EKS authentication token",
	RunE:  runGetToken,
}

func init() {
	rootCmd.AddCommand(eksCmd)
	eksCmd.AddCommand(getTokenCmd)
	
	getTokenCmd.Flags().StringVar(&eksRegion, "region", "", "AWS region (required)")
	getTokenCmd.Flags().StringVar(&clusterName, "cluster-name", "", "EKS cluster name (required)")
	getTokenCmd.Flags().StringVar(&outputFormat, "output", "json", "Output format (must be 'json')")
	getTokenCmd.MarkFlagRequired("region")
	getTokenCmd.MarkFlagRequired("cluster-name")
}

func runGetToken(cmd *cobra.Command, args []string) error {
	if outputFormat != "json" {
		return fmt.Errorf("only 'json' is accepted for --output")
	}

	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		return fmt.Errorf("AWS_PROFILE environment variable is required")
	}

	cachePath, err := kubeCacheFilePath(clusterName)
	if err != nil {
		return fmt.Errorf("failed to get cache path: %w", err)
	}

	// Try to use cached token if valid
	if cred, ok := tryReadValidCache(cachePath); ok {
		fmt.Println(string(cred))
		return nil
	}

	ctx := context.Background()

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(eksRegion),
		config.WithSharedConfigProfile(profile),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create STS client
	stsClient := sts.NewFromConfig(cfg)

	// Create presigner with custom presigner implementation
	presigner := sts.NewPresignClient(stsClient, func(po *sts.PresignOptions) {
		po.Presigner = &customPresigner{
			signer:  v4.NewSigner(),
			expires: maxTokenDuration,
		}
	})

	// Presign GetCallerIdentity request with custom header
	presignedReq, err := presigner.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{},
		func(opts *sts.PresignOptions) {
			opts.ClientOptions = append(opts.ClientOptions, func(o *sts.Options) {
				o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
					return stack.Build.Add(middleware.BuildMiddlewareFunc(
						"AddEKSHeader",
						func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
							req, ok := in.Request.(*smithyhttp.Request)
							if ok {
								req.Header.Set("x-k8s-aws-id", clusterName)
							}
							return next.HandleBuild(ctx, in)
						},
					), middleware.Before)
				})
			})
		},
	)
	if err != nil {
		return fmt.Errorf("failed to presign STS request: %w", err)
	}

	token := "k8s-aws-v1." + encodeBase64Url(presignedReq.URL)
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
		return fmt.Errorf("failed to marshal ExecCredential: %w", err)
	}

	// Write to disk
	if err := os.WriteFile(cachePath, out, 0600); err != nil {
		return fmt.Errorf("failed to write ExecCredential to file: %w", err)
	}

	fmt.Println(string(out))
	return nil
}

// customPresigner wraps v4.Signer to set custom expiry duration
type customPresigner struct {
	signer  *v4.Signer
	expires time.Duration
}

func (p *customPresigner) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*v4.SignerOptions),
) (url string, signedHeader http.Header, err error) {
	// Add X-Amz-Expires query parameter before presigning
	query := r.URL.Query()
	query.Set("X-Amz-Expires", fmt.Sprintf("%d", int64(p.expires.Seconds())))
	r.URL.RawQuery = query.Encode()
	
	return p.signer.PresignHTTP(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
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
func kubeCacheFilePath(cluster string) (string, error) {
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

	filename := fmt.Sprintf("eks-token-%s.json", cluster)
	return filepath.Join(cacheDir, filename), nil
}
