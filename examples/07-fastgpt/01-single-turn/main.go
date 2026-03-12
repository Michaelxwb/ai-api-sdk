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
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	providerName = "fastgpt_local"
	modelName    = "fastgpt"
	promptText   = "请用简短的50字说明 FastGPT 是什么？"
)

type singleTurnCase struct {
	stream bool
	detail bool
	label  string
}

func main() {
	cli := client.New()
	if err := loadLocalConfig(cli); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	ctx := context.Background()

	fmt.Println("=== FastGPT 单轮对话示例 ===")
	fmt.Println("说明：演示 stream/detail 四种组合；detail/variables 通过 ExtraBody 注入。")

	cases := []singleTurnCase{
		{stream: false, detail: false, label: "stream=false, detail=false"},
		{stream: false, detail: true, label: "stream=false, detail=true"},
		{stream: true, detail: false, label: "stream=true, detail=false"},
		{stream: true, detail: true, label: "stream=true, detail=true"},
	}

	for i, c := range cases {
		fmt.Printf("\n场景%d：=========================%s=========================\n", i+1, c.label)
		runSingleTurn(cli, ctx, c)
		time.Sleep(2 * time.Second)
	}
}

func runSingleTurn(cli *client.Client, ctx context.Context, c singleTurnCase) {
	variables := map[string]any{
		"team":    "demo",
		"channel": "fastgpt-single",
		"case":    c.label,
	}

	cred, pc, err := resolveProvider(cli, providerName)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	pc.ExtraBody = map[string]any{
		"detail":    c.detail,
		"variables": variables,
	}

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithHistoryMode(client.HistoryNone),
		client.WithConversationMode(client.ConversationModeLocalHistory),
	)

	req := base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
		StartNewChat: true,
	}

	if c.stream {
		fmt.Print("回答: ")
		text, err := chatStream(ctx, sess, req)
		if err != nil {
			log.Printf("Error: %v", err)
			return
		}
		if text == "" {
			fmt.Printf("(空响应)\n")
		}
		return
	}

	resp, err := sess.Chat(ctx, req)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}
	fmt.Printf("回答: %s\n", resp.Text)
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
