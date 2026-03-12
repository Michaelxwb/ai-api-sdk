package main

import (
	"context"
	"fmt"
	"log"
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
	localProviderName  = "fastgpt_local"
	remoteProviderName = "fastgpt_remote"
	modelName          = "fastgpt"
	firstPrompt        = "请记住：我最喜欢的水果是苹果。"
	secondPrompt       = "我最喜欢的水果是什么？"
)

func main() {
	cli := client.New()
	if err := loadLocalConfig(cli); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	ctx := context.Background()

	fmt.Println("=== FastGPT 多轮对话示例 ===")
	fmt.Println("说明：同时展示 local_history 与 remote_session 模式。")

	// ========================================
	// 场景1：local_history（HistoryAuto 本地加载历史）
	// ========================================
	fmt.Println("场景1：=========================local_history（HistoryAuto）=========================")
	exampleLocalHistory(cli, ctx)
	fmt.Println()

	// ========================================
	// 场景2：remote_session（不拼接历史，由服务端维护）
	// ========================================
	fmt.Println("场景2：=========================remote_session（ChatStream）=========================")
	exampleRemoteSession(cli, ctx)
	fmt.Println()

	time.Sleep(2 * time.Second)
}

func exampleLocalHistory(cli *client.Client, ctx context.Context) {
	store, err := sessionstore.NewFileStore(filepath.Join("examples", "sessions.json"))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	variables := map[string]any{
		"team":    "demo",
		"channel": "fastgpt-local",
	}

	sess, err := newSessionWithExtra(cli, localProviderName, false, variables,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryAuto),
		client.WithConversationMode(client.ConversationModeLocalHistory),
		client.WithAutoID(),
	)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Print("第一次回答: ")
	text1, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
		StartNewChat: false,
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
		StartNewChat: false,
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("\n(本地历史自动拼接完成，回答长度: %d)\n", len(text2))
	_ = text1
}

func exampleRemoteSession(cli *client.Client, ctx context.Context) {
	store, err := sessionstore.NewFileStore(filepath.Join("examples", "sessions.json"))
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	variables := map[string]any{
		"team":    "demo",
		"channel": "fastgpt-remote",
	}

	sess, err := newSessionWithExtra(cli, remoteProviderName, true, variables,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
		client.WithConversationMode(client.ConversationModeRemoteSession),
	)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Print("第一次回答: ")
	text1, err := chatStream(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: firstPrompt,
		}},
		StartNewChat: false,
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
		StartNewChat: false,
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

func newSessionWithExtra(cli *client.Client, providerName string, detail bool, variables map[string]any, opts ...client.SessionOption) (*client.Session, error) {
	cred, pc, err := resolveProvider(cli, providerName)
	if err != nil {
		return nil, err
	}
	pc.ExtraBody = map[string]any{
		"detail":    detail,
		"variables": variables,
	}
	return cli.NewSessionWith(cred, pc, opts...), nil
}

func resolveProvider(cli *client.Client, name string) (*auth.Credential, *config.ProviderConfig, error) {
	if cli == nil || cli.Config == nil {
		return nil, nil, fmt.Errorf("client config not loaded")
	}
	pc := cli.Config.FindProvider(name)
	if pc == nil {
		return nil, nil, fmt.Errorf("provider %s not configured", name)
	}
	if cli.AuthMgr == nil {
		return nil, nil, fmt.Errorf("auth manager not initialized")
	}
	cred, err := resolveCredential(cli, pc)
	if err != nil {
		return nil, nil, err
	}
	return cred, cloneProviderConfig(pc), nil
}

func resolveCredential(cli *client.Client, pc *config.ProviderConfig) (*auth.Credential, error) {
	if pc.AuthRef != "" {
		return cli.AuthMgr.Get(pc.AuthRef)
	}
	providerName := pc.Type
	if providerName == "" {
		providerName = pc.Name
	}
	cred, _, err := cli.AuthMgr.Resolve(providerName)
	return cred, err
}

func cloneProviderConfig(pc *config.ProviderConfig) *config.ProviderConfig {
	if pc == nil {
		return nil
	}
	clone := *pc
	if pc.Headers != nil {
		headers := make(map[string]string, len(pc.Headers))
		for k, v := range pc.Headers {
			headers[k] = v
		}
		clone.Headers = headers
	}
	if pc.ExtraBody != nil {
		extra := make(map[string]any, len(pc.ExtraBody))
		for k, v := range pc.ExtraBody {
			extra[k] = v
		}
		clone.ExtraBody = extra
	}
	return &clone
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
