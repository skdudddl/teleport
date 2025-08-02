package watcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gravitational/teleport/api/types"
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

func StartPodWatcher(ctx context.Context, webhookURL string) error {
	log.Println("Waiting for Teleport to be ready...")
	time.Sleep(10 * time.Second)

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

	log.Println("Pod watcher started...")

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*corev1.Pod)
		if !ok {
			continue
		}

		eventType := event.Type
		podName := pod.GetName()
		namespace := pod.GetNamespace()
		labels := pod.GetLabels()

		kubernetesResource := types.KubernetesResource{
			Kind:      "pods",
			Name:      podName,
			Namespace: namespace,
		}

		PodResourceWithLabels := PodResourceWithLabels{
			KubernetesResource: kubernetesResource,
			Labels:             labels,
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
