package einotool

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	agenttool "github.com/sizolity/worldline/agent/tool"
)

// ToEinoTools converts agent/tool.Tool values to eino InvokableTools.
func ToEinoTools(tools []agenttool.Tool) []tool.InvokableTool {
	out := make([]tool.InvokableTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, &einoToolWrapper{inner: t})
	}
	return out
}

type einoToolWrapper struct {
	inner agenttool.Tool
}

func (w *einoToolWrapper) Info(_ context.Context) (*schema.ToolInfo, error) {
	info := w.inner.Info()
	toolInfo := &schema.ToolInfo{
		Name: info.Name,
		Desc: info.Description,
	}
	if info.Parameters != nil {
		toolInfo.ParamsOneOf = convertParams(info.Parameters)
	}
	return toolInfo, nil
}

func (w *einoToolWrapper) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	return w.inner.Invoke(ctx, arguments)
}

func convertParams(params any) *schema.ParamsOneOf {
	m, ok := params.(map[string]any)
	if !ok {
		return nil
	}
	props, _ := m["properties"].(map[string]any)
	if props == nil {
		return nil
	}
	required, _ := m["required"].([]any)
	reqSet := make(map[string]bool, len(required))
	for _, r := range required {
		if s, ok := r.(string); ok {
			reqSet[s] = true
		}
	}

	paramInfos := make(map[string]*schema.ParameterInfo, len(props))
	for name, prop := range props {
		p, ok := prop.(map[string]any)
		if !ok {
			continue
		}
		pType, _ := p["type"].(string)
		pDesc, _ := p["description"].(string)
		info := &schema.ParameterInfo{
			Type:     schema.DataType(pType),
			Desc:     pDesc,
			Required: reqSet[name],
		}
		if items, ok := p["items"].(map[string]any); ok {
			elemType, _ := items["type"].(string)
			info.ElemInfo = &schema.ParameterInfo{Type: schema.DataType(elemType)}
		}
		paramInfos[name] = info
	}
	return schema.NewParamsOneOfByParams(paramInfos)
}

var _ tool.InvokableTool = (*einoToolWrapper)(nil)
