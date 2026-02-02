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

	_ "github.com/Michaelxwb/ai-api-sdk/provider"
)

const (
	providerName = "openai"
	modelName    = "gpt-3.5-turbo"
)

func main() {
	cli := client.New()
	ctx := context.Background()

	fmt.Println("=== 连通性测试示例 ===\n")

	// ========================================
	// 场景1：Test() - 本地配置模式
	// ========================================
	fmt.Println("场景1：Test() - 本地配置模式")
	testLocalConfig(cli, ctx)

	// ========================================
	// 场景2：TestWith() - 平台集成模式
	// ========================================
	fmt.Println("\n场景2：TestWith() - 平台集成模式")
	testPlatformIntegration(cli, ctx)
}

func testLocalConfig(cli *client.Client, ctx context.Context) {
	if err := loadLocalConfig(cli); err != nil {
		log.Printf("加载配置失败: %v", err)
		return
	}

	result, err := cli.Test(ctx, providerName, &client.TestOptions{
		Model:   modelName,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		log.Printf("测试失败: %v", err)
		return
	}

	fmt.Printf("✓ 测试通过\n")
	fmt.Printf("  延迟: %v\n", result.Latency)
	fmt.Printf("  响应: %s\n", result.Response.Text)
}

func testPlatformIntegration(cli *client.Client, ctx context.Context) {
	cred := &auth.Credential{
		ID:       "test-cred",
		Provider: "openai",
		AuthType: auth.AuthTypeAPIKey,
		APIKey:   "sk-...",
	}

	pc := &config.ProviderConfig{
		Name:    "openai",
		Type:    "openai",
		BaseURL: "https://api.openai.com/v1",
	}

	result, err := cli.TestWith(ctx, cred, pc, &client.TestOptions{
		Model:   modelName,
		Timeout: 10 * time.Second,
	})
	if err != nil {
		log.Printf("测试失败: %v", err)
		return
	}

	fmt.Printf("✓ 测试通过\n")
	fmt.Printf("  延迟: %v\n", result.Latency)
	fmt.Printf("  响应: %s\n", result.Response.Text)
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
