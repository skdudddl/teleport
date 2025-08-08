package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type PodResourceWithLabels struct {
	types.KubernetesResource
	Labels map[string]string `json:"labels,omitempty"`
}

type UserSession struct {
	Username   string
	RoleSet    services.RoleSet
	UserTraits map[string][]string
	Cluster    string
	ProxyURL   string
	LoginTime  time.Time
}

var (
	currentUserSession *UserSession
	sessionMutex       sync.RWMutex
)

var (
	cancelFunc context.CancelFunc
	cancelOnce sync.Once
)

func UpdateUserSession(session *UserSession) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	currentUserSession = session
	log.Printf("💻 [UpdateUserSession] Session set for user: %s", session.Username)
	log.Printf("💻 [UpdateUserSession] Session set for user: %s", session.Cluster)
	log.Printf("💻 [UpdateUserSession] Session set for user: %s", session.RoleSet)
	log.Printf("💻 [UpdateUserSession] Session set for user: %s", session.UserTraits)
}

func GetCurrentUserSession() *UserSession {
	sessionMutex.RLock()
	defer sessionMutex.RUnlock()
	if currentUserSession == nil {
		log.Println("💻 [GetCurrentUserSession] currentUserSession is nil")
	} else {
		log.Printf("💻 [GetCurrentUserSession] currentUserSession for user: %s", currentUserSession.Username)
	}
	return currentUserSession
}

func StringsToRoleSet(roles []string) services.RoleSet {
	roleSet := make(services.RoleSet, 0, len(roles))
	for _, roleName := range roles {
		roleSet = append(roleSet, &types.RoleV6{
			Metadata: types.Metadata{
				Name: roleName,
			},
		})
	}
	return roleSet
}

func StartPodWatcher(ctx context.Context, webhookURL string) error {

	log.Println("🚀 [StartPodWatcher] Pod watcher started")

	session := GetCurrentUserSession()
	if session == nil {
		log.Println("🚫 [StartPodWatcher] No user session available")
		return nil
	}

	log.Printf("✅ [StartPodWatcher] Found user session for: %s", session.Username)

	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = home + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("failed to get in-cluster config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	watcher, err := clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to pod watcher: %w", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		eventType := event.Type
		podName := pod.GetName()
		namespace := pod.GetNamespace()
		labels := pod.GetLabels()

		/*kubernetesResource := types.KubernetesResource{
			Kind:      "pods",
			Name:      podName,
			Namespace: namespace,
		}

		PodResourceWithLabels := PodResourceWithLabels{
			KubernetesResource: kubernetesResource,
			Labels:             labels,
		}*/

		PodResourceWithLabels := PodResourceWithLabels{
			KubernetesResource: types.KubernetesResource{
				Kind:      "pods",
				Name:      podName,
				Namespace: namespace,
			},
			Labels: labels,
		}

		userSession := GetCurrentUserSession()
		if userSession != nil {
			log.Printf("Checking pod access for user: %s", userSession.Username)
			err := MatchPodAccessAndNotify(
				ctx,
				PodResourceWithLabels,
				userSession.RoleSet,
				userSession.UserTraits,
				userSession.Cluster,
				webhookURL,
			)
			if err != nil {
				log.Printf("Failed to match pod access: %v", err)
			}
		} else {
			log.Println("🚨 No user session available, skipping access check")
		}

		resourceJSON, err := json.MarshalIndent(PodResourceWithLabels, "", " ")
		if err != nil {
			log.Printf("Failed to marshal PodResourceWithLabels: %v", err)
			continue
		}

		labelStr := ""
		for k, v := range labels {
			labelStr += fmt.Sprintf("%s=%s ", k, v)
		}

		message := fmt.Sprintf(
			"*[%s]* PodResourceWithLabels:\n```json\n%s\n```\nTime: %s",
			eventType, string(resourceJSON), time.Now().Format(time.RFC3339),
		)

		err = sendSlackNotification(webhookURL, message)
		if err != nil {
			log.Printf("Failed to send Slack notification: %v", err)
		}

		log.Printf("Pod event: %s - %s/%s with labels: %v", eventType, namespace, podName, labels)
	}

	return nil
}
