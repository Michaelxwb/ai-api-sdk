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
	providerName = "vllm_local"
	modelName    = "claude-sonnet-4-5-20250929"
	promptText   = "什么是Rust语言？"
	streamOutput = true
)

func main() {
	cli := client.New()
	if err := loadLocalConfig(cli); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	ctx := context.Background()

	fmt.Println("=== 单轮对话示例 ===\n")

	// ========================================
	// 场景1：无SessionStore（最简单）
	// ========================================
	//fmt.Println("场景1：=========================无SessionStore=========================")
	example1_NoStore(cli, ctx)
	time.Sleep(10 * time.Second)

	// ========================================
	// 场景2：Memory SessionStore（内存审计）
	// ========================================
	//fmt.Println("\n场景2：=========================Memory SessionStore=========================")
	//example2_MemoryStore(cli, ctx)
	//time.Sleep(10 * time.Second)

	// ========================================
	// 场景3：File SessionStore
	// ========================================
	//fmt.Println("\n场景3：=========================File SessionStore=========================")
	//example3_FileStore(cli, ctx)
	//time.Sleep(10 * time.Second)

	// ========================================
	// 场景4：SQLite SessionStore
	// ========================================
	//fmt.Println("\n场景4：=========================SQLite SessionStore=========================")
	//example4_SQLiteStore(cli, ctx)
	//time.Sleep(10 * time.Second)

	// ========================================
	// 场景5：MySQL SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景5：=========================MySQL SessionStore=========================")
	// example5_MySQLStore(cli, ctx)
	//time.Sleep(10 * time.Second)

	// ========================================
	// 场景6：PostgreSQL SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景6：=========================PostgreSQL SessionStore=========================")
	// example6_PostgreSQLStore(cli, ctx)
	//time.Sleep(10 * time.Second)

	// ========================================
	// 场景7：Redis SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景7：=========================Redis SessionStore=========================")
	// example7_RedisStore(cli, ctx)
}

func example1_NoStore(cli *client.Client, ctx context.Context) {
	sess := cli.NewSession(
		providerName,
		client.WithHistoryMode(client.HistoryNone),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
		// 单轮隔离：确保本次对话不依赖历史
		StartNewChat: true,
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}

	// 再来一次单轮对话，仍保持隔离（不依赖上一轮）
	if streamOutput {
		fmt.Print("第二次回答: ")
	}
	text2, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: "再问一次：你还记得我刚才的问题吗？",
		}},
		StartNewChat: true,
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
		client.WithHistoryMode(client.HistoryNone), // 单轮：仅持久化，不加载历史
		client.WithAutoID(),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
		// 需要单轮隔离可改为 true（会跳过历史加载）
		StartNewChat: false,
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("会话ID: %s (已保存到Memory)\n", sess.ID())
}

func example3_FileStore(cli *client.Client, ctx context.Context) {
	store := sessionstore.NewFile(sessionstore.FileConfig{
		BaseDir: "examples",
	})

	sess := cli.NewSession(
		providerName,
		client.WithStore(store),
		client.WithMeta(map[string]string{
			"user_id":   "u-123",
			"chat_room": "room-456",
		}),
		client.WithHistoryMode(client.HistoryNone),
		client.WithAutoID(),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
		// 需要单轮隔离可改为 true（会跳过历史加载）
		StartNewChat: false,
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("会话ID: %s (已保存到 examples/)\n", sess.ID())
}

func example4_SQLiteStore(cli *client.Client, ctx context.Context) {
	store, err := sessionstore.NewSQLite(sessionstore.SQLiteConfig{
		DSN: "examples/sessions.db",
	})
	if err != nil {
		log.Printf("Error creating SQLite store: %v", err)
		return
	}
	defer func() { _ = store.Close() }()

	sess := cli.NewSession(
		providerName,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
		client.WithAutoID(),
		client.WithMeta(map[string]string{
			"user_id":   "x-123",
			"chat_room": "xxxx-456",
		}),
	)

	if streamOutput {
		fmt.Print("回答: ")
	}
	text, err := chat(ctx, sess, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
		// 需要单轮隔离可改为 true（会跳过历史加载）
		StartNewChat: false,
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if !streamOutput {
		fmt.Printf("回答: %s\n", text)
	}
	fmt.Printf("会话ID: %s (已保存到SQLite)\n", sess.ID())
}

func example5_MySQLStore(_ *client.Client, _ context.Context) {
	// 需要先配置 MySQL DSN，例如：
	// "user:password@tcp(localhost:3306)/sessions?parseTime=true"
	// store, err := sessionstore.NewMySQL(sessionstore.MySQLConfig{DSN: "..."})
	// if err != nil {
	// 	log.Printf("Error creating MySQL store: %v", err)
	// 	return
	// }
	// defer store.Close()
}

func example6_PostgreSQLStore(_ *client.Client, _ context.Context) {
	// 需要先配置 PostgreSQL DSN，例如：
	// "postgres://user:password@localhost:5432/sessions?sslmode=disable"
	// store, err := sessionstore.NewPostgreSQL(sessionstore.PostgreSQLConfig{DSN: "..."})
	// if err != nil {
	// 	log.Printf("Error creating PostgreSQL store: %v", err)
	// 	return
	// }
	// defer store.Close()
}

func example7_RedisStore(_ *client.Client, _ context.Context) {
	// 需要先配置 Redis 连接，例如：
	// "redis://localhost:6379/0"
	// store, err := sessionstore.NewRedis(sessionstore.RedisConfig{Addr: "localhost:6379"})
	// if err != nil {
	// 	log.Printf("Error creating Redis store: %v", err)
	// 	return
	// }
	// defer store.Close()
}

// chat 根据 streamOutput 常量选择流式或非流式调用，返回响应文本
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
