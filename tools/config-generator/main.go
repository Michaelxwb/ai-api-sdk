package generator

import (
	"flag"
	"fmt"
	"os"
)

// RunCLI 解析命令行参数并生成配置文档。
func RunCLI(args []string) error {
	fs := flag.NewFlagSet("config-generator", flag.ContinueOnError)
	input := fs.String("input", "", "输入请求响应包 Markdown 路径")
	output := fs.String("output", "", "输出 Markdown 路径")
	platform := fs.String("platform", "", "平台名称（可选，用于多平台输入场景）")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *input == "" || *output == "" {
		return fmt.Errorf("必须提供 --input 和 --output")
	}
	if *platform != "" {
		return GenerateConfigForPlatform(*input, *output, *platform)
	}
	return GenerateConfig(*input, *output)
}

// Main 作为 CLI 入口的包装函数。
func Main() {
	if err := RunCLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "配置生成失败:", err)
		os.Exit(1)
	}
}
