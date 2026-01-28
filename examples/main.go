package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"ai-api-sdk/auth"
	"ai-api-sdk/client"
	"ai-api-sdk/config"
	"ai-api-sdk/provider"
)

func main() {
	configPath := "examples/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("load config failed: %v\n", err)
		os.Exit(1)
	}

	store := auth.NewFileStore(cfg.Auth.Store.Path)
	store.Encrypted = cfg.Auth.Store.Encryption.Enabled || cfg.Auth.Store.Encrypted
	store.MasterKeyEnv = cfg.Auth.Store.Encryption.MasterKeyEnv
	store.MasterKeyFile = cfg.Auth.Store.Encryption.MasterKeyFile
	if cfg.Auth.Store.Encryption.KDFParams.N > 0 {
		store.ScryptParams.N = cfg.Auth.Store.Encryption.KDFParams.N
		store.ScryptParams.R = cfg.Auth.Store.Encryption.KDFParams.R
		store.ScryptParams.P = cfg.Auth.Store.Encryption.KDFParams.P
		store.ScryptParams.KeyLen = cfg.Auth.Store.Encryption.KDFParams.KeyLen
	}

	mgr, err := auth.NewManager(store, &auth.RoundRobinSelector{})
	if err != nil {
		fmt.Printf("init auth manager failed: %v\n", err)
		os.Exit(1)
	}

	// 将 config 中声明的凭证注册到 Manager（覆盖 store 中的同 ID 记录）
	for _, cred := range cfg.Credentials {
		mgr.Register(cred)
	}

	cli := client.NewClient(cfg, mgr)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, err := cli.Chat(ctx, "vllm_local", provider.ChatRequest{
		Model:    "minimaxai/minimax-m2.1",
		Messages: []provider.Message{{Role: "user", Content: "安全评估平台要注意哪些关键点？"}},
	})
	if err != nil {
		fmt.Printf("vllm_local error: %v\n", err)
	} else {
		fmt.Printf("vllm_local response: %s\n", resp.Text)
	}

	// Local OpenAI-compatible (llama.cpp / vLLM)
	//resp, err := cli.Chat(ctx, "llama_local", provider.ChatRequest{
	//	Model:    "llama3",
	//	Messages: []provider.Message{{Role: "user", Content: "你好，介绍一下你的能力"}},
	//})
	//if err != nil {
	//	fmt.Printf("llama_local error: %v\n", err)
	//} else {
	//	fmt.Printf("llama_local response: %s\n", resp.Text)
	//}

	// OpenAI cloud
	//resp, err := cli.Chat(ctx, "openai_cloud", provider.ChatRequest{
	//	Model:    "gpt-4o-mini",
	//	Messages: []provider.Message{{Role: "user", Content: "安全评估平台要注意哪些关键点？"}},
	//})
	//if err != nil {
	//	fmt.Printf("openai_cloud error: %v\n", err)
	//} else {
	//	fmt.Printf("openai_cloud response: %s\n", resp.Text)
	//}

	//resp, err := cli.Chat(ctx, "deepseek", provider.ChatRequest{
	//	Model:    "deepseek-chat",
	//	Messages: []provider.Message{{Role: "user", Content: "安全评估平台要注意哪些关键点？"}},
	//})
	//if err != nil {
	//	fmt.Printf("deepseek error: %v\n", err)
	//} else {
	//	fmt.Printf("deepseek response: %s\n", resp.Text)
	//}

	// Custom gateway (New API / One API style)
	//resp, err := cli.Chat(ctx, "customer_gateway", provider.ChatRequest{
	//	Model:    "claude-4.5-sonnet",
	//	Messages: []provider.Message{{Role: "user", Content: "测试自定义网关接入"}},
	//})
	//if err != nil {
	//	fmt.Printf("customer_gateway error: %v\n", err)
	//} else {
	//	fmt.Printf("customer_gateway response: %s\n", resp.Text)
	//}
}
