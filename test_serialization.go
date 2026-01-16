// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/claude-code-go/internal/api"
)

// 测试 Content 序列化，确保内部字段不被发送到 API
func main() {
	content := api.Content{
		Type: api.ContentTypeText,
		Text: "Hello, world!",
		// 设置内部字段
		Pruned:   true,
		PrunedAt: time.Now(),
	}

	// 序列化
	data, err := json.Marshal(content)
	if err != nil {
		fmt.Printf("❌ Serialization failed: %v\n", err)
		return
	}

	jsonStr := string(data)
	fmt.Printf("✅ Serialized JSON: %s\n", jsonStr)

	// 检查是否包含内部字段
	var decoded map[string]interface{}
	json.Unmarshal(data, &decoded)

	if _, exists := decoded["pruned"]; exists {
		fmt.Println("❌ ERROR: 'pruned' field should not be serialized!")
		return
	}

	if _, exists := decoded["pruned_at"]; exists {
		fmt.Println("❌ ERROR: 'pruned_at' field should not be serialized!")
		return
	}

	fmt.Println("✅ SUCCESS: Internal fields are not serialized")

	// 测试完整的消息
	message := api.Message{
		Role: api.RoleUser,
		Content: []api.Content{
			{
				Type:     api.ContentTypeText,
				Text:     "Test message",
				Pruned:   true,
				PrunedAt: time.Now(),
			},
		},
	}

	msgData, _ := json.MarshalIndent(message, "", "  ")
	fmt.Printf("\n✅ Full message JSON:\n%s\n", string(msgData))
}
