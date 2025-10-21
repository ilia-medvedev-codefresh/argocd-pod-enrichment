package argocdresourcetracking

import (
	"argocd-pod-enrichment-webhook/pkg/consts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"strings"
)

type ArgoCDTrackingInfo struct {
	ApplicationName      string
	InstallationID       string
	ApplicationNamespace string
}

func ExtractArgoCDTrackingInfo(resource unstructured.Unstructured) *ArgoCDTrackingInfo {
	annotations := resource.GetAnnotations()

	var (
		applicationName      string
		installationID       string
		applicationNamespace string
	)

	trackingID, hasTrackingID := annotations[consts.ArgoCDTrackingIDAnnotation]

	if hasTrackingID {

		// Split trackingID by ':' and take the first member
		parts := strings.Split(trackingID, ":")

		firstPart := ""

		if len(parts) > 0 {
			firstPart = parts[0]
		}

		appNameAndNamespaceSlice := strings.Split(firstPart, "_")

		if len(appNameAndNamespaceSlice) > 0 {
			applicationName = appNameAndNamespaceSlice[0]
		}

		if len(appNameAndNamespaceSlice) > 1 {
			applicationNamespace = appNameAndNamespaceSlice[len(appNameAndNamespaceSlice)-1]
		}
	} else {
		// Check if ArgoCDTrackingLabelEnvironmentVariable is set in the environment
		labelEnv := os.Getenv(consts.ArgoCDTrackingLabelEnvironmentVariable)

		labels := resource.GetLabels()

		if labelEnv != "" {
			if val, ok := labels[labelEnv]; ok {
				applicationName = val
			}
		} else {
			if val, ok := labels[consts.ArgoCDDefaultTrackingLabel]; ok {
				applicationName = val
			}
		}
	}

	if applicationName != "" {
		return &ArgoCDTrackingInfo{
		ApplicationName:      applicationName,
		InstallationID:       installationID,
		ApplicationNamespace: applicationNamespace,
		}
	} else {
		return nil
	}
}
