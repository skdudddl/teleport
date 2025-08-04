package watcher

import (
	"bytes"
	"context"
	"fmt"
	"github.com/goccy/go-json"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"io"
	"log"
	"net/http"
)

// MatchPodAccessAndNotify checks if the user is allowed to access the specified pod.
// If access is denied, it logs the denial and optionally sends an alert (e.g., Slack, email, webhook).
// If access is granted, it logs the successful access.
func MatchPodAccessAndNotify(ctx context.Context, pod types.KubernetesResource, cluster types.KubeCluster, roleSet services.RoleSet, userTraits map[string][]string, webhookURL string) error {
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
	err := roleSet.CheckAccessToPod(ctx, cluster, pod, userTraits)
	// CheckAccessToPod checks if the user has access to the specified pod.
	if err != nil {
		if trace.IsAccessDenied(err) {
			// Access denied — log the event
			log.Printf("Access DENIED to pod %q: %v", pod.Name, err)

			if webhookURL != "" {
				msg := fmt.Sprintf("Access DENIED to pod: %s in namespace: %s", pod.Name, pod.Namespace)
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
	log.Printf("Access ALLOWED to pod %q", pod.Name)
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
