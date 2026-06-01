package einochat

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/sizolity/worldline/agent/chat"
)

// Adapter wraps an eino ToolCallingChatModel behind the agent/chat interfaces.
type Adapter struct {
	CM model.ToolCallingChatModel
}

var (
	_ chat.Model       = (*Adapter)(nil)
	_ chat.StreamModel = (*Adapter)(nil)
)

func (a *Adapter) Generate(ctx context.Context, msgs []chat.Message, opts ...chat.Option) (chat.Message, error) {
	einoMsgs := ToEinoMessages(msgs)
	resp, err := a.CM.Generate(ctx, einoMsgs, toEinoOptions(opts)...)
	if err != nil {
		return chat.Message{}, err
	}
	return FromEinoMessage(resp), nil
}

func (a *Adapter) Stream(ctx context.Context, msgs []chat.Message, opts ...chat.Option) (*chat.StreamReader, error) {
	einoMsgs := ToEinoMessages(msgs)
	sr, err := a.CM.Stream(ctx, einoMsgs, toEinoOptions(opts)...)
	if err != nil {
		return nil, err
	}
	ch := make(chan chat.Message, 32)
	go func() {
		defer close(ch)
		for {
			chunk, err := sr.Recv()
			if err != nil {
				return
			}
			ch <- FromEinoMessage(chunk)
		}
	}()
	return &chat.StreamReader{Ch: ch}, nil
}

// ToEinoMessages converts agent/chat messages to eino schema messages.
func ToEinoMessages(msgs []chat.Message) []*schema.Message {
	out := make([]*schema.Message, len(msgs))
	for i, m := range msgs {
		em := &schema.Message{Role: toEinoRole(m.Role), Content: m.Content}
		for _, tc := range m.ToolCalls {
			em.ToolCalls = append(em.ToolCalls, schema.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: schema.FunctionCall{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		if m.ToolCallID != "" {
			em.ToolCallID = m.ToolCallID
		}
		out[i] = em
	}
	return out
}

// FromEinoMessage converts an eino schema message to agent/chat.Message.
func FromEinoMessage(m *schema.Message) chat.Message {
	msg := chat.Message{
		Role:       fromEinoRole(m.Role),
		Content:    m.Content,
		ToolCallID: m.ToolCallID,
	}
	for _, tc := range m.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, chat.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	return msg
}

func toEinoRole(r chat.Role) schema.RoleType {
	switch r {
	case chat.RoleSystem:
		return schema.System
	case chat.RoleUser:
		return schema.User
	case chat.RoleAssistant:
		return schema.Assistant
	case chat.RoleTool:
		return schema.Tool
	default:
		return schema.User
	}
}

func fromEinoRole(r schema.RoleType) chat.Role {
	switch r {
	case schema.System:
		return chat.RoleSystem
	case schema.User:
		return chat.RoleUser
	case schema.Assistant:
		return chat.RoleAssistant
	case schema.Tool:
		return chat.RoleTool
	default:
		return chat.RoleAssistant
	}
}

func toEinoOptions(_ []chat.Option) []model.Option {
	return nil
}
