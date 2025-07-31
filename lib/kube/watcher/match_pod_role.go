package watcher

import (
	"context"
	"log"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// MatchPodAccessAndNotify checks access to the pod and triggers alert if denied.
func MatchPodAccessAndNotify(ctx context.Context, pod types.KubernetesResource, cluster types.KubeCluster, roleSet services.RoleSet, userTraits map[string][]string) error {
	err := roleSet.CheckAccessToPod(ctx, cluster, pod, userTraits)
	if err != nil {
		if trace.IsAccessDenied(err) {
			// Access denied error handling
			log.Printf("🚨 Access DENIED to pod %q: %v", pod.Name, err)

			// 👉 여기에 Slack, Email, Webhook 등 알림 로직 추가 가능
			// notify.SendAccessAlert(pod, user, err)

			return nil // 접근 거부는 정상 처리
		}
		// ❗️기타 에러 처리
		return trace.Wrap(err)
	}

	// Access granted, log the success
	log.Printf("✅ Access ALLOWED to pod %q", pod.Name)
	return nil
}
