package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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
	cluster := flag.String("cluster", "", "EKS cluster name (required)")
	region := flag.String("region", "", "AWS region (required)")
	flag.Parse()

	if *cluster == "" || *region == "" {
		flag.Usage()
		os.Exit(1)
	}

	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		fmt.Fprintln(os.Stderr, "AWS_PROFILE environment variable is required")
		os.Exit(1)
	}

	cachePath, err := kubeCacheFilePath(*cluster)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get cache path: %v\n", err)
		os.Exit(1)
	}

	// Try to use cached token if valid
	if cred, ok := tryReadValidCache(cachePath); ok {
		fmt.Println(string(cred))
		return
	}

	// Set up AWS session
	sess, err := session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region: aws.String(*region),
		},
		Profile: profile,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create AWS session: %v\n", err)
		os.Exit(1)
	}

	// Get caller identity
	stsSvc := sts.New(sess)
	req, _ := stsSvc.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
	req.HTTPRequest.Header.Add("x-k8s-aws-id", *cluster)

	// Presign the request with max duration (15m)
	urlStr, err := req.Presign(int64(maxTokenDuration.Seconds()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to presign STS request: %v\n", err)
		os.Exit(1)
	}

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