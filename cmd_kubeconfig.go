package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	kubeconfigPath string
)

var kubeconfigCmd = &cobra.Command{
	Use:   "kubeconfig",
	Short: "Kubeconfig operations",
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Display kubeconfig contexts in a table format",
	RunE:  runShowKubeconfig,
}

func init() {
	rootCmd.AddCommand(kubeconfigCmd)
	kubeconfigCmd.AddCommand(showCmd)
	
	showCmd.Flags().StringVar(&kubeconfigPath, "kubeconfig", "", "Path to kubeconfig file")
}

func runShowKubeconfig(cmd *cobra.Command, args []string) error {
	// Load kubeconfig
	config, err := loadKubeconfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	// Get contexts and current context
	contexts := config.Contexts
	currentContext := config.CurrentContext

	// Create sorted list of context names
	var contextNames []string
	for name := range contexts {
		contextNames = append(contextNames, name)
	}
	sort.Strings(contextNames)

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	
	// Print header with borders
	fmt.Fprintln(w, "+------------------+------------------+------------------+")
	fmt.Fprintln(w, "| CONTEXT NAME\t| CLUSTER\t| USER\t|")
	fmt.Fprintln(w, "+------------------+------------------+------------------+")

	// Print each context
	for _, name := range contextNames {
		context := contexts[name]
		
		// Determine context name prefix
		displayName := name
		if name == currentContext {
			displayName = "*" + name
		}
		
		// Validate cluster reference
		clusterName := context.Cluster
		if _, exists := config.Clusters[clusterName]; !exists {
			clusterName = "(not found)"
		}
		
		// Validate user reference
		userName := context.AuthInfo
		if _, exists := config.AuthInfos[userName]; !exists {
			userName = "(not found)"
		}
		
		fmt.Fprintf(w, "| %s\t| %s\t| %s\t|\n", displayName, clusterName, userName)
	}
	
	// Print bottom border
	fmt.Fprintln(w, "+------------------+------------------+------------------+")
	
	w.Flush()
	return nil
}

func loadKubeconfig() (*clientcmdapi.Config, error) {
	// Set up loading rules
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	
	// If --kubeconfig flag is provided, use it
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	// Otherwise, loadingRules will check $KUBECONFIG, then ~/.kube/config
	
	// Load and merge configs
	config, err := loadingRules.Load()
	if err != nil {
		return nil, err
	}
	
	return config, nil
}

// Helper function to get kubeconfig paths for display/error messages
func getKubeconfigPaths() []string {
	if kubeconfigPath != "" {
		return []string{kubeconfigPath}
	}
	
	if envPath := os.Getenv("KUBECONFIG"); envPath != "" {
		return filepath.SplitList(envPath)
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}
	return []string{filepath.Join(homeDir, ".kube", "config")}
}
