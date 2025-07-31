package watcher

import (
	"context"
	"log"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// MatchPodAccessAndNotify checks if the user is allowed to access the specified pod.
// If access is denied, it logs the denial and optionally sends an alert (e.g., Slack, email, webhook).
// If access is granted, it logs the successful access.
func MatchPodAccessAndNotify(ctx context.Context, pod types.KubernetesResource, cluster types.KubeCluster, roleSet services.RoleSet, userTraits map[string][]string) error {
	err := roleSet.CheckAccessToPod(ctx, cluster, pod, userTraits)
	if err != nil {
		if trace.IsAccessDenied(err) {
			// Access denied — log the event
			log.Printf("Access DENIED to pod %q: %v", pod.Name, err)

			// Optional: Send alert notification via Slack, email, webhook, etc.
			// notify.SendAccessAlert(pod, user, err)

			return nil // Denial is a handled, expected outcome
		}
		// nexpected error — return wrapped error
		return trace.Wrap(err)
	}

	// Access granted — log the event
	log.Printf("Access ALLOWED to pod %q", pod.Name)
	return nil
}
