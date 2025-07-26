package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

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

		labelStr := ""
		for k, v := range labels {
			labelStr += fmt.Sprintf("%s=%s ", k, v)
		}

		message := fmt.Sprintf(
			"*[%s]* Pod: `%s`\nNamespace: `%s`\nLabels: `%s`\nTime: %s",
			eventType, podName, namespace, labelStr, time.Now().Format(time.RFC3339),
		)

		err := sendSlackNotification(webhookURL, message)
		if err != nil {
			log.Printf("Failed to send Slack notification: %v", err)
		}
	}

	return nil
}
