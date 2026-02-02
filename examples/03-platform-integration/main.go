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
	modelName  = "gpt-3.5-turbo"
	promptText = "什么是Go语言？"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 平台集成示例（NewSessionWith） ===\n")

	// 构造凭证（模拟平台从数据库获取）
	cred := &auth.Credential{
		ID:       "user-123-openai",
		Provider: "openai",
		AuthType: auth.AuthTypeAPIKey,
		APIKey:   "sk-...",
	}

	pc := &config.ProviderConfig{
		Name:    "openai",
		Type:    "openai",
		BaseURL: "https://api.openai.com/v1",
	}

	// ========================================
	// 场景1：无SessionStore（最简单）
	// ========================================
	fmt.Println("场景1：无SessionStore")
	example1_NoStore(cli, ctx, cred, pc)

	// ========================================
	// 场景2：Memory SessionStore（内存审计）
	// ========================================
	fmt.Println("\n场景2：Memory SessionStore")
	example2_MemoryStore(cli, ctx, cred, pc)

	// ========================================
	// 场景3：File SessionStore
	// ========================================
	fmt.Println("\n场景3：File SessionStore")
	example3_FileStore(cli, ctx, cred, pc)

	// ========================================
	// 场景4：SQLite SessionStore
	// ========================================
	fmt.Println("\n场景4：SQLite SessionStore")
	example4_SQLiteStore(cli, ctx, cred, pc)

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

	resp, err := sess.Chat(ctx, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("回答: %s\n", resp.Text)
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

	resp, err := sess.Chat(ctx, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("回答: %s\n", resp.Text)
	fmt.Printf("会话ID: %s (已保存到Memory)\n", sess.ID())
}

func example3_FileStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store := sessionstore.NewFile(sessionstore.FileConfig{
		BaseDir: "/tmp/sessions",
	})

	sess := cli.NewSessionWith(
		cred,
		pc,
		client.WithStore(store),
		client.WithHistoryMode(client.HistoryNone),
		client.WithAutoID(),
	)

	resp, err := sess.Chat(ctx, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("回答: %s\n", resp.Text)
	fmt.Printf("会话ID: %s (已保存到 /tmp/sessions/)\n", sess.ID())
}

func example4_SQLiteStore(cli *client.Client, ctx context.Context, cred *auth.Credential, pc *config.ProviderConfig) {
	store, err := sessionstore.NewSQLite(sessionstore.SQLiteConfig{
		DSN: "file:/tmp/sessions.db",
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

	resp, err := sess.Chat(ctx, base.ChatRequest{
		Model: modelName,
		Messages: []base.Message{{
			Role:    "user",
			Content: promptText,
		}},
	})
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	fmt.Printf("回答: %s\n", resp.Text)
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
