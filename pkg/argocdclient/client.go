package argocdclient

import (
	"fmt"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient"
)

// NewClient creates a new ArgoCD API client using the specified server address and auth token
func NewClient(serverAddr, authToken string) (apiclient.Client, error) {
	conn, err := apiclient.NewClient(&apiclient.ClientOptions{
		ServerAddr: serverAddr,
		PlainText:  true, // set to false if using TLS
		AuthToken:  authToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create ArgoCD API client: %w", err)
	}
	return conn, nil
}
