package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	modelName    = "fastgpt"
	firstPrompt  = "请记住：我最常用的编辑器是 VS Code。"
	secondPrompt = "我最常用的编辑器是什么？"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== FastGPT 平台集成示例（NewSessionWith） ===")
	fmt.Println("说明：使用 NewSessionWith + Credential + ProviderConfig 演示 local_history / remote_session。")

	cred := &auth.Credential{
		ID:       "user-123-fastgpt",
		Provider: "fastgpt",
		AuthType: auth.AuthTypeAPIKey,
		APIKey:   "fastgpt-TOKEN",
	}

	pcLocal := &config.ProviderConfig{
		Name:    "fastgpt_local",
		Type:    "fastgpt",
		BaseURL: "https://api.fastgpt.in",
		Path:    "/api/v1/chat/completions",
		ExtraBody: map[string]any{
			"detail": true,
			"variables": map[string]any{
				"team":    "demo",
				"channel": "platform-local",
			},
		},
	}

	pcRemote := &config.ProviderConfig{
		Name:    "fastgpt_remote",
		Type:    "fastgpt",
		BaseURL: "https://api.fastgpt.in",
		Path:    "/api/v1/chat/completions",
		ExtraBody: map[string]any{
			"detail": false,
			"variables": map[string]any{
				"team":    "demo",
				"channel": "platform-remote",
			},
		},
	}

	// ========================================
	// 场景1：local_history（HistoryAuto 本地拼接历史）
	// ========================================
	fmt.Println("场景1：=========================local_history=========================")
	exampleLocalHistory(cli, ctx, cred, pcLocal)

	// ========================================
	// 场景2：remote_session（不拼接历史，服务端维护）
	// ========================================
	fmt.Println("\n场景2：=========================remote_session=========================")
	exampleRemoteSession(cli, ctx, cred, pcRemote)
}

func exampleLocalHistory(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store, err := sessionstore.NewFileStore(filepath.Join("examples", "sessions.json"))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeLocalHistory),
		client.WithAutoID(),
	)

	fmt.Print("第一次回答: ")
	text1, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("\n(会话ID: %s)\n", sess.ID())

	fmt.Print("第二次回答: ")
	text2, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: secondPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("\n(本地历史已自动拼接，回答长度: %d)\n", len(text2))
	_ = text1
}

func exampleRemoteSession(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store, err := sessionstore.NewFileStore(filepath.Join("examples", "sessions.json"))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	fmt.Print("第一次回答: ")
	text1, err := chatStream(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("(chatId: %s)\n", sess.ID())

	fmt.Print("第二次回答: ")
	text2, err := chatStream(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: secondPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("(remote_session 未拼接历史，回答长度: %d)\n", len(text2))
	_ = text1
}

func chat(ctx context.Context, sess *client.Session, req base.ChatRequest) (string, error) {
	resp, err := sess.Chat(ctx, req)
	if err != nil {
		return "", err
	}
	if resp.Text != "" {
		fmt.Print(resp.Text)
	}
	return resp.Text, nil
}

func chatStream(ctx context.Context, sess *client.Session, req base.ChatRequest) (string, error) {
	stream, err := sess.ChatStream(ctx, req)
	if err != nil {
		return "", err
	}
	var fullText string
	for chunk := range stream {
		if chunk.Error != nil {
			return fullText, chunk.Error
		}
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
			fullText += chunk.Text
		}
	}
	fmt.Println()
	return fullText, nil
}
