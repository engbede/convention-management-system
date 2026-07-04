package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type TermiiRequest struct {
	To      string `json:"to"`
	From    string `json:"from"`
	SMS     string `json:"sms"`
	Type    string `json:"type"`
	Channel string `json:"channel"`
	ApiKey  string `json:"api_key"`
}

func SendSMS(phone string, message string) error {
	key := os.Getenv("TERMII_API_KEY")

	fmt.Printf("API Key length: %d\n", len(key))

	if len(key) >= 8 {
		fmt.Printf("API Key prefix: %s...\n", key[:8])
	}
	fmt.Println("TERMII_API_KEY:", os.Getenv("TERMII_API_KEY"))
	fmt.Println("TERMII_SENDER:", os.Getenv("TERMII_SENDER"))
	request := TermiiRequest{
		To:      phone,
		From:    os.Getenv("TERMII_SENDER"),
		SMS:     message,
		Type:    "plain",
		Channel: "generic",
		ApiKey:  os.Getenv("TERMII_API_KEY"),
	}

	body, err := json.Marshal(request)

	if err != nil {
		return err
	}

	resp, err := http.Post(
		"https://api.ng.termii.com/api/sms/send",
		"application/json",
		bytes.NewBuffer(body),
	)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	fmt.Println("========== TERMII RESPONSE ==========")
	fmt.Println(string(responseBody))
	fmt.Println("=====================================")

	if resp.StatusCode != http.StatusOK {

		return fmt.Errorf(
			"termii returned %s",
			resp.Status,
		)
	}

	return nil
}
