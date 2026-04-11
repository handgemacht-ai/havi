package webhook

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"
)

type Webhook struct {
	url    string
	client *http.Client
}

func NewWebhook(url string) *Webhook {
	if url == "" {
		return nil
	}
	return &Webhook{
		url:    url,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (wh *Webhook) Fire(_ context.Context, data []byte) {
	if wh == nil {
		return
	}
	go func() {
		resp, err := wh.client.Post(wh.url, "application/json", bytes.NewReader(data))
		if err != nil {
			log.Printf("webhook error=%v url=%s", err, wh.url)
			return
		}
		resp.Body.Close()
	}()
}
