package watcher

import (
	"context"
	"os"
	"testing"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

func TestMatchPodAccessAndNotify(t *testing.T) {
	err := godotenv.Load(".env")
	require.NoError(t, err, "failed to load .env.test")

	webhookURL := os.Getenv("WEBHOOK_URI")
	require.NotEmpty(t, webhookURL, "WEBHOOK_URI must be set in .env.test")

	ctx := context.Background()

	cluster, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name:   "test-cluster",
			Labels: map[string]string{"env": "dev"},
		},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	pod := types.KubernetesResource{
		Kind:      "pods",
		Name:      "nginx",
		Namespace: "default",
	}

	denyRole, err := types.NewRole("deny-role", types.RoleSpecV6{
		Deny: types.RoleConditions{
			KubernetesLabels: types.Labels{"env": []string{"dev"}},
			KubernetesResources: []types.KubernetesResource{
				pod,
			},
		},
	})
	require.NoError(t, err)

	roleSet := services.RoleSet{denyRole}
	userTraits := map[string][]string{}

	err = MatchPodAccessAndNotify(ctx, pod, cluster, roleSet, userTraits, webhookURL)
	require.NoError(t, err)
}
