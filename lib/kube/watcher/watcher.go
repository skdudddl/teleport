package watcher

import (
	"context"
	"fmt"
	"log"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func StartPodWatcher(ctx context.Context, webhookURL string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	watcher, err := clientset.CoreV1().Pods("").Watch(ctx, v1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to pod watcher: %w", err)
	}
	defer watcher.Stop()

	log.Println("Pod watcher started...")

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*v1.Pod)
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
