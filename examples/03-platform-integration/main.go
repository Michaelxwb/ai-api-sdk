package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Michaelxwb/ai-api-sdk/auth"
	"github.com/Michaelxwb/ai-api-sdk/client"
	"github.com/Michaelxwb/ai-api-sdk/config"
	"github.com/Michaelxwb/ai-api-sdk/examples/sessionstore"
	"github.com/Michaelxwb/ai-api-sdk/provider/base"

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	modelName    = "Qwen3-32B-FP8"
	promptText   = "请用简短的100字，什么是Go语言？"
	streamOutput = true
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 平台集成示例（NewSessionWith） ===")

	// 构造凭证（模拟平台从数据库获取）
	cred := &auth.Credential{
		ID:       "user-123-openai",
		Provider: "openai_compat",
		AuthType: auth.AuthTypeBearerToken,
		APIKey:   "sk-TOKEN",
	}

	pc := &config.ProviderConfig{
		Name:    "vllm_local",
		Type:    "openai_compat",
		BaseURL: "http://10.6.193.48:30090/v1",
	}

	// ========================================
	// 场景1：无SessionStore（最简单）
	// ========================================
	fmt.Println("场景1：=========================无SessionStore=========================")
	example1_NoStore(cli, ctx, cred, pc)

	// ========================================
	// 场景2：Memory SessionStore（内存审计）
	// ========================================
	//fmt.Println("\n场景2：=========================Memory SessionStore=========================")
	//example2_MemoryStore(cli, ctx, cred, pc)

	// ========================================
	// 场景3：File SessionStore
	// ========================================
	//fmt.Println("\n场景3：=========================File SessionStore=========================")
	//example3_FileStore(cli, ctx, cred, pc)

	// ========================================
	// 场景4：SQLite SessionStore
	// ========================================
	//fmt.Println("\n场景4：=========================SQLite SessionStore=========================")
	//example4_SQLiteStore(cli, ctx, cred, pc)

	// ========================================
	// 场景5：MySQL SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景5：MySQL SessionStore")
	// example5_MySQLStore(cli, ctx, cred, pc)

	// ========================================
	// 场景6：PostgreSQL SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景6：PostgreSQL SessionStore")
	// example6_PostgreSQLStore(cli, ctx, cred, pc)

	// ========================================
	// 场景7：Redis SessionStore（需要配置）
	// ========================================
	// fmt.Println("\n场景7：Redis SessionStore")
	// example7_RedisStore(cli, ctx, cred, pc)
}

func example1_NoStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	sess := cli.NewSessionWith(
		cred,
		pc,
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
}

func example2_MemoryStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store := sessionstore.NewMemory()

	sess := cli.NewSessionWith(
		cred,
		pc,
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

func example3_FileStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store := sessionstore.NewFile(sessionstore.FileConfig{
		BaseDir: "examples",
	})

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
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

func example4_SQLiteStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store, err := sessionstore.NewSQLite(sessionstore.SQLiteConfig{
		DSN: "examples/sessions.db",
	})
	if err != nil {
		log.Printf("Error creating SQLite store: %v", err)
		return
	}
	defer func() { _ = store.Close() }()

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
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
	fmt.Printf("会话ID: %s (已保存到SQLite)\n", sess.ID())
}

func example5_MySQLStore(_ *client.Client, _ context.Context, _ *auth.Credential, _ *config.ProviderConfig) {
	// 需要先配置 MySQL DSN，例如：
	// "user:password@tcp(localhost:3306)/sessions?parseTime=true"
	// store, err := sessionstore.NewMySQL(sessionstore.MySQLConfig{DSN: "..."})
	// if err != nil {
	// 	log.Printf("Error creating MySQL store: %v", err)
	// 	return
	// }
	// defer store.Close()
}

func example6_PostgreSQLStore(_ *client.Client, _ context.Context, _ *auth.Credential, _ *config.ProviderConfig) {
	// 需要先配置 PostgreSQL DSN，例如：
	// "postgres://user:password@localhost:5432/sessions?sslmode=disable"
	// store, err := sessionstore.NewPostgreSQL(sessionstore.PostgreSQLConfig{DSN: "..."})
	// if err != nil {
	// 	log.Printf("Error creating PostgreSQL store: %v", err)
	// 	return
	// }
	// defer store.Close()
}

func example7_RedisStore(_ *client.Client, _ context.Context, _ *auth.Credential, _ *config.ProviderConfig) {
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
