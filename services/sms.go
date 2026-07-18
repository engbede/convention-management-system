package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type smsRequest struct {
	Messages []smsMessage `json:"messages"`
}

type smsMessage struct {
	From         string           `json:"from"`
	Destinations []smsDestination `json:"destinations"`
	Text         string           `json:"text"`
}

type smsDestination struct {
	To string `json:"to"`
}

func SendSMS(phone, message string) error {

	baseURL := os.Getenv("INFOBIP_BASE_URL")
	apiKey := os.Getenv("INFOBIP_API_KEY")
	sender := os.Getenv("INFOBIP_SENDER")

	if baseURL == "" {
		return fmt.Errorf("INFOBIP_BASE_URL is not configured")
	}

	if apiKey == "" {
		return fmt.Errorf("INFOBIP_API_KEY is not configured")
	}

	if sender == "" {
		return fmt.Errorf("INFOBIP_SENDER is not configured")
	}

	payload := smsRequest{
		Messages: []smsMessage{
			{
				From: sender,
				Destinations: []smsDestination{
					{
						To: phone,
					},
				},
				Text: message,
			},
		},
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		baseURL+"/sms/2/text/advanced",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "App "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 300 {
		return fmt.Errorf(
			"Infobip error (%d): %s",
			resp.StatusCode,
			string(body),
		)
	}

	fmt.Println("Infobip response:")
	fmt.Println(string(body))

	return nil
}
