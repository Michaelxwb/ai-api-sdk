package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	providerName      = "ragflow"
	firstPrompt       = "请记住：我最喜欢的编程语言是 Go。"
	secondPrompt      = "我最喜欢的编程语言是什么？"
	streamOutput bool = true
)

func main() {
	cli := client.New()
	if err := loadLocalConfig(cli); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cli.HTTP = &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	ctx := context.Background()

	fmt.Println("=== RAGFlow 多轮对话示例 ===")
	fmt.Println("说明：RAGFlow 的 session_id 由服务端生成，SDK 会自动提取并复用。")
	fmt.Println("注意：chat_id 需通过配置的 extra_body 或 ExtraBody 传入。")

	// ========================================
	// 场景1：无SessionStore（手动传递历史）
	// ========================================
	fmt.Println("场景1：无SessionStore")
	example1_NoStore(cli, ctx)

	// ========================================
	// 场景2：Memory SessionStore（自动加载历史）
	// ========================================
	fmt.Println("\n场景2：Memory SessionStore")
	example2_MemoryStore(cli, ctx)

	// ========================================
	// 场景3：File SessionStore
	// ========================================
	fmt.Println("\n场景3：File SessionStore")
	example3_FileStore(cli, ctx)

	// ========================================
	// 场景4：SQLite SessionStore
	// ========================================
	fmt.Println("\n场景4：SQLite SessionStore")
	example4_SQLiteStore(cli, ctx)
}

func example1_NoStore(cli *client.Client, ctx context.Context) {
	sess := cli.NewSession(
		providerName,
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	if streamOutput {
		fmt.Print("第一次回答: ")
	}
	text1, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第一次回答: %s\n", text1)
	}
	fmt.Printf("session_id: %s (自动提取)\n", sess.ID())

	// 无 SessionStore 时，需要手动携带历史上下文
	history := []base.Message{
		{Role: "user", Content: firstPrompt},
		{Role: "assistant", Content: text1},
	}

	if streamOutput {
		fmt.Print("第二次回答: ")
	}
	text2, err := chat(ctx, sess, base.ChatRequest{
		Messages: append(history, base.Message{Role: "user", Content: secondPrompt}),
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第二次回答: %s\n", text2)
	}
}

func example2_MemoryStore(cli *client.Client, ctx context.Context) {
	store := sessionstore.NewMemory()

	sess := cli.NewSession(
		providerName,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	if streamOutput {
		fmt.Print("第一次回答: ")
	}
	text1, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第一次回答: %s\n", text1)
	}
	fmt.Printf("session_id: %s (自动提取)\n", sess.ID())

	if streamOutput {
		fmt.Print("第二次回答: ")
	}
	text2, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: secondPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第二次回答: %s\n", text2)
	}
	fmt.Printf("会话ID: %s (已保存到Memory)\n", sess.ID())
}

func example3_FileStore(cli *client.Client, ctx context.Context) {
	store := sessionstore.NewFile(sessionstore.FileConfig{
		BaseDir: "/tmp/sessions",
	})

	sess := cli.NewSession(
		providerName,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	if streamOutput {
		fmt.Print("第一次回答: ")
	}
	text1, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第一次回答: %s\n", text1)
	}
	fmt.Printf("session_id: %s (自动提取)\n", sess.ID())

	if streamOutput {
		fmt.Print("第二次回答: ")
	}
	text2, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: secondPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第二次回答: %s\n", text2)
	}
	fmt.Printf("会话ID: %s (已保存到 /tmp/sessions/)\n", sess.ID())
}

func example4_SQLiteStore(cli *client.Client, ctx context.Context) {
	store, err := sessionstore.NewSQLite(sessionstore.SQLiteConfig{
		DSN: "file:/tmp/sessions.db",
	})
	if err != nil {
		log.Printf("Error creating SQLite store: %v", err)
		return
	}
	defer func() { _ = store.Close() }()

	sess := cli.NewSession(
		providerName,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)

	if streamOutput {
		fmt.Print("第一次回答: ")
	}
	text1, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第一次回答: %s\n", text1)
	}
	fmt.Printf("session_id: %s (自动提取)\n", sess.ID())

	if streamOutput {
		fmt.Print("第二次回答: ")
	}
	text2, err := chat(ctx, sess, base.ChatRequest{
		Messages: []base.Message{{
			Role:    "user",
			Content: secondPrompt,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	if !streamOutput {
		fmt.Printf("第二次回答: %s\n", text2)
	}
	fmt.Printf("会话ID: %s (已保存到SQLite)\n", sess.ID())
}

func chat(ctx context.Context, sess *client.Session, req base.ChatRequest) (string, error) {
	if !streamOutput {
		resp, err := sess.Chat(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Text, nil
	}

	stream, err := sess.ChatStream(ctx, req)
	if err != nil {
		return "", err
	}

	var fullText string
	for chunk := range stream {
		if chunk.Error != nil {
			return "", chunk.Error
		}
		if chunk.Text != "" {
			fmt.Print(chunk.Text)
			fullText += chunk.Text
		}
	}
	fmt.Println()
	return fullText, nil
}

func loadLocalConfig(cli *client.Client) error {
	cfgPath := findConfigPath()
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return err
	}
	cfg.Auth.Store.Path = resolvePath(cfgPath, cfg.Auth.Store.Path)

	authStore := auth.NewFileStore(cfg.Auth.Store.Path)
	applyAuthStoreConfig(authStore, cfg)

	mgr, err := auth.NewManager(authStore, &auth.RoundRobinSelector{})
	if err != nil {
		return err
	}
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	cli.Config = cfg
	cli.AuthMgr = mgr
	return nil
}

func findConfigPath() string {
	candidates := []string{
		"examples/config.example.yaml",
		"../config.example.yaml",
		"config.example.yaml",
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return "examples/config.example.yaml"
}

func resolvePath(cfgPath, target string) string {
	if target == "" || filepath.IsAbs(target) {
		return target
	}
	baseDir := filepath.Dir(cfgPath)
	return filepath.Join(baseDir, target)
}

func applyAuthStoreConfig(store *auth.FileStore, cfg *config.Config) {
	store.Encrypted = cfg.Auth.Store.Encryption.Enabled || cfg.Auth.Store.Encrypted
	store.MasterKeyEnv = cfg.Auth.Store.Encryption.MasterKeyEnv
	store.MasterKeyFile = cfg.Auth.Store.Encryption.MasterKeyFile
	if cfg.Auth.Store.Encryption.KDFParams.N > 0 {
		store.ScryptParams.N = cfg.Auth.Store.Encryption.KDFParams.N
		store.ScryptParams.R = cfg.Auth.Store.Encryption.KDFParams.R
		store.ScryptParams.P = cfg.Auth.Store.Encryption.KDFParams.P
		store.ScryptParams.KeyLen = cfg.Auth.Store.Encryption.KDFParams.KeyLen
	}
}
