package watcher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/goccy/go-json"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// MatchPodAccessAndNotify checks if the user is allowed to access the specified pod.
// If access is denied, it logs the denial and optionally sends an alert (e.g., Slack, email, webhook).
// If access is granted, it logs the successful access.
func MatchPodAccessAndNotify(
	ctx context.Context,
	podResource PodResourceWithLabels,
	roleSet services.RoleSet,
	userTraits map[string][]string,
	clusterName string,
	webhookURL string) error {
	//podResource := types.KubernetesResource{
	//	Kind:      "pods",
	//	Namespace: pod.Namespace,
	//	Name:      pod.Name,
	//	Verbs:     []string{"get"},
	//}
	//
	//cluster, err := types.NewKubernetesClusterV3(types.Metadata{
	//	Name:   clusterName,
	//	Labels: map[string]string{"env": "dev"}, // or actual label from metadata
	//}, types.KubernetesClusterSpecV3{})
	//if err != nil {
	//	log.Error(err)
	//	return
	//}

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   clusterName,
		Labels: map[string]string{"env": "production"}, // 필요에 따라 변경 가능
	}, types.KubernetesClusterSpecV3{})
	if err != nil {
		log.Printf("Failed to create cluster object: %v", err)
		return trace.Wrap(err)
	}

	kubernetesResource := types.KubernetesResource{
		Kind:      podResource.Kind,
		Namespace: podResource.Namespace,
		Name:      podResource.Name,
		Verbs:     []string{"get", "list"}, // Default verbs for pod access check
	}

	// Create a dummy cluster for access check
	// TODO: Get actual cluster information from current session
	/*cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   "default-cluster",                      // This should come from user session
		Labels: map[string]string{"env": "production"}, // This should come from actual cluster metadata
	}, types.KubernetesClusterSpecV3{})
	if err != nil {
		log.Printf("Failed to create cluster object: %v", err)
		return trace.Wrap(err)
	}*/

	log.Printf("Checking access for pod: %s/%s with labels: %v, clusterName: %s",
		podResource.Namespace, podResource.Name, podResource.Labels, cluster.GetName())

	err = roleSet.CheckAccessToPod(ctx, cluster, kubernetesResource, userTraits)
	// CheckAccessToPod checks if the user has access to the specified pod.
	if err != nil {
		if trace.IsAccessDenied(err) {
			// Access denied — log the event
			log.Printf("Access DENIED to pod %s/%s: %v", podResource.Namespace, podResource.Name, err)

			if webhookURL != "" {
				msg := fmt.Sprintf("Access DENIED to pod: %s in namespace: %s", podResource.Name, podResource.Namespace)
				if err := sendWebhookAlert(webhookURL, msg); err != nil {
					log.Printf("❗️ Failed to send webhook alert: %v", err)
				}
			}

			return nil // Denial is a handled, expected outcome
		}
		// nexpected error — return wrapped error
		return trace.Wrap(err)
	}

	// Access granted — log the event
	log.Printf("Access ALLOWED to pod %q", podResource.Name)
	return nil
}

// sendWebhookAlert sends a POST request to the given webhook URL with the alert message
func sendWebhookAlert(webhookURL string, message string) error {
	payload := map[string]string{"text": message}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("webhook failed: %s", string(body))
	}

	return nil
}
