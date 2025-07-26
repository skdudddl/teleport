package watcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type slackPayload struct {
	Text string `json:"text"`
}

func sendSlackNotification(webhookURL, message string) error {
	webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		return nil
	}

	payload := slackPayload{
		Text: message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack responded with non-OK status: %s", resp.Status)
	}

	return nil
}
