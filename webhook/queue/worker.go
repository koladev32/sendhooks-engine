package queue

import (
	"context"
	"fmt"
	"log"
	"time"
	"webhook/logging"
	"webhook/sender"

	redisClient "webhook/redis"
)

func ProcessWebhooks(ctx context.Context, webhookQueue chan redisClient.WebhookPayload) {
	for payload := range webhookQueue {
		go func(p redisClient.WebhookPayload) {
			backoffTime := time.Second  // starting backoff time
			maxBackoffTime := time.Hour // maximum backoff time
			retries := 0
			maxRetries := 5

			for {
				err := sender.SendWebhook(p.Data, p.Url, p.WebhookId, p.SecretHash)
				if err == nil {
					break
				}
				logging.WebhookLogger(logging.ErrorType, fmt.Errorf("error sending webhook: %s", err))

				retries++
				if retries >= maxRetries {
					logging.WebhookLogger(logging.WarningType, fmt.Errorf("max retries reached. Giving up on webhook: %s", p.WebhookId))
					break
				}

				time.Sleep(backoffTime)

				// Double the backoff time for the next iteration, capped at the max
				backoffTime *= 2
				log.Println(backoffTime)
				if backoffTime > maxBackoffTime {
					backoffTime = maxBackoffTime
				}
			}
		}(payload)
	}
}

func sendWebhookWithRetries(payload redisClient.WebhookPayload) {
	if err := retryWithExponentialBackoff(payload, 5, 1*time.Second, time.Hour); err != nil {
		log.Println("Failed to send webhook after maximum retries. WebhookID:", payload.WebhookId)
	}
}

func retryWithExponentialBackoff(payload redisClient.WebhookPayload, maxRetries int, initialBackoff, maxBackoff time.Duration) error {
	retries := 0
	backoffTime := initialBackoff

	for retries < maxRetries {
		err := sender.SendWebhook(payload.Data, payload.Url, payload.WebhookId)
		if err == nil {
			return nil
		}

		log.Println("Error sending webhook:", err)

		backoffTime = calculateBackoff(backoffTime, maxBackoff)
		retries++

		time.Sleep(backoffTime)
	}

	return log.Println("Maximum retries reached")
}

func calculateBackoff(currentBackoff, maxBackoff time.Duration) time.Duration {

	nextBackoff := currentBackoff * 2

	if nextBackoff > maxBackoff {
		return maxBackoff
	}

	return nextBackoff
}
