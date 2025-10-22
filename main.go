package main

import (
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"argocd-pod-enrichment-webhook/cmd/webhook"
)

var rootCmd = &cobra.Command{
   Use:   "argocd-pod-enrichment",
   Short: "ArgoCD Pod Enrichment",
   Run: func(cmd *cobra.Command, args []string) {
	   fmt.Println("Available subcommands:")
	   for _, c := range cmd.Commands() {
		   fmt.Printf("  %s\t%s\n", c.Name(), c.Short)
	   }
	   os.Exit(0)
   },
}

func init() {
	rootCmd.AddCommand(webhook.WebhookCmd)
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
