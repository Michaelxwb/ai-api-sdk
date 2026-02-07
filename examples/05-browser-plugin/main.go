package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"
	"github.com/Michaelxwb/ai-api-sdk/provider/plugin"
)

type locatorWrapper struct {
	Locators plugin.ElementLocators `json:"locators"`
}

func main() {
	endpoint := flag.String("endpoint", "ws://localhost:8080/ws?role=client", "plugin platform websocket endpoint")
	configID := flag.String("config", "470836a3-bcd3-4fc5-b767-1030fa4f8389", "plugin platform config ID")
	locatorsPath := flag.String("locators", "examples/05-browser-plugin/locators.json", "path to locators JSON file")
	text := flag.String("text", "Hello", "message to send")
	sessionID := flag.String("session", "", "optional session ID")
	startNewChat := flag.Bool("new", false, "start a new chat before sending")
	stream := flag.Bool("stream", false, "enable streaming output")
	authToken := flag.String("token", "", "optional auth token for websocket endpoint")
	timeout := flag.Duration("timeout", 120*time.Second, "request timeout")
	flag.Parse()

	if *configID == "" {
		*configID = strings.TrimSpace(os.Getenv("PLUGIN_CONFIG_ID"))
	}
	if *locatorsPath == "" {
		*locatorsPath = strings.TrimSpace(os.Getenv("PLUGIN_LOCATORS_PATH"))
	}
	if *endpoint == "" {
		*endpoint = strings.TrimSpace(os.Getenv("PLUGIN_WS_ENDPOINT"))
	}
	if *authToken == "" {
		*authToken = strings.TrimSpace(os.Getenv("PLUGIN_AUTH_TOKEN"))
	}

	if *endpoint == "" {
		log.Fatal("missing -endpoint (or PLUGIN_WS_ENDPOINT)")
	}
	if *configID == "" {
		log.Fatal("missing -config (or PLUGIN_CONFIG_ID)")
	}
	if *locatorsPath == "" {
		log.Fatal("missing -locators (or PLUGIN_LOCATORS_PATH)")
	}

	locators, err := loadLocators(*locatorsPath)
	if err != nil {
		log.Fatalf("load locators: %v", err)
	}

	cfg := plugin.Config{
		Endpoint:       *endpoint,
		AuthToken:      *authToken,
		ConfigID:       *configID,
		Locators:       locators,
		RequestTimeout: *timeout,
	}
	opts := []client.SessionOption{}
	if *sessionID != "" {
		opts = append(opts, client.WithID(*sessionID))
	}
	sess, err := plugin.NewSession(cfg, opts...)
	if err != nil {
		log.Fatalf("init session: %v", err)
	}

	ctx := context.Background()
	if *timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}

	req := base.ChatRequest{
		Messages:     []base.Message{{Role: "user", Content: *text}},
		StartNewChat: *startNewChat,
	}

	if *stream {
		ch, err := sess.ChatStream(ctx, req)
		if err != nil {
			log.Fatalf("chat stream failed: %v", err)
		}
		for chunk := range ch {
			if chunk.Error != nil {
				log.Fatalf("stream error: %v", chunk.Error)
			}
			if chunk.Text != "" {
				fmt.Print(chunk.Text)
			}
			if chunk.Done {
				break
			}
		}
		fmt.Println()
		return
	}

	resp, err := sess.Chat(ctx, req)
	if err != nil {
		log.Fatalf("chat failed: %v", err)
	}
	fmt.Printf("reply: %s\n", resp.Text)
	if resp.SessionID != "" {
		fmt.Printf("session_id: %s\n", resp.SessionID)
	}
}

func loadLocators(path string) (*plugin.ElementLocators, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapped locatorWrapper
	if err := json.Unmarshal(data, &wrapped); err == nil {
		if !locatorsEmpty(wrapped.Locators) {
			return &wrapped.Locators, nil
		}
	}
	var locators plugin.ElementLocators
	if err := json.Unmarshal(data, &locators); err != nil {
		return nil, err
	}
	if locatorsEmpty(locators) {
		return nil, fmt.Errorf("locators file is empty")
	}
	return &locators, nil
}

func locatorsEmpty(locators plugin.ElementLocators) bool {
	return locatorEmpty(locators.Input) &&
		locatorEmpty(locators.SendButton) &&
		locatorEmpty(locators.ReplyArea)
}

func locatorEmpty(locator plugin.ElementLocator) bool {
	return strings.TrimSpace(locator.Selector) == "" && strings.TrimSpace(locator.XPath) == ""
}
