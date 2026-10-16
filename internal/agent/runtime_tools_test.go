package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

func TestSkillInvokerReturnsFullInstruction(t *testing.T) {
	skills := []store.Skill{
		{Nome: "vendas", Descricao: "fluxo de vendas", InstrucaoCompleta: "INSTRUCOES COMPLETAS DE VENDAS"},
	}
	tool := skillInvokerTool(skills)
	if tool.name != "invocar_skill" {
		t.Fatalf("nome da tool = %q", tool.name)
	}
	out := tool.invoke(context.Background(), map[string]interface{}{"skill_nome": "vendas"})
	if !strings.Contains(out, "INSTRUCOES COMPLETAS DE VENDAS") {
		t.Fatalf("instrução completa não foi devolvida: %q", out)
	}
	missing := tool.invoke(context.Background(), map[string]interface{}{"skill_nome": "inexistente"})
	if !strings.Contains(missing, "não encontrada") {
		t.Fatalf("skill desconhecida deveria retornar aviso, obtido: %q", missing)
	}
}

func TestSkillsPromptSectionOnlyExposesMetadata(t *testing.T) {
	section := skillsPromptSection([]store.Skill{
		{Nome: "suporte", Descricao: "ajuda técnica", InstrucaoCompleta: "SEGREDO INTERNO"},
	})
	if !strings.Contains(section, "[suporte]") || !strings.Contains(section, "ajuda técnica") {
		t.Fatalf("seção de skills sem metadados: %q", section)
	}
	if strings.Contains(section, "SEGREDO INTERNO") {
		t.Fatalf("seção de skills não pode expor a instrução completa: %q", section)
	}
}

func TestMergeToolDefsSkipsDuplicates(t *testing.T) {
	base := []llm.Tool{{Type: "function", Function: llm.ToolFunction{Name: "echo"}}}
	extras := []runtimeTool{
		{name: "echo", def: llm.Tool{Type: "function", Function: llm.ToolFunction{Name: "echo"}}},
		{name: "invocar_skill", def: llm.Tool{Type: "function", Function: llm.ToolFunction{Name: "invocar_skill"}}},
	}
	merged := mergeToolDefs(base, extras)
	if len(merged) != 2 {
		t.Fatalf("merge deveria manter 2 tools, obtido %d", len(merged))
	}
	if merged[0].Function.Name != "echo" || merged[1].Function.Name != "invocar_skill" {
		t.Fatalf("ordem/conteúdo inesperado: %+v", merged)
	}
}

func TestNormalizeMCPInputSchema(t *testing.T) {
	schema := normalizeMCPInputSchema(nil)
	if schema["type"] != "object" {
		t.Fatalf("schema nulo deveria virar object: %+v", schema)
	}
	existing := map[string]interface{}{"properties": map[string]interface{}{"x": map[string]interface{}{"type": "string"}}}
	normalized := normalizeMCPInputSchema(existing)
	if normalized["type"] != "object" {
		t.Fatalf("type deveria ser preenchido: %+v", normalized)
	}
}

func TestMCPInvokerToolCallsServer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&msg)
		if len(msg.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		var result interface{}
		switch msg.Method {
		case "initialize":
			result = map[string]interface{}{"protocolVersion": "2024-11-05", "serverInfo": map[string]interface{}{"name": "fake", "version": "1"}}
		case "tools/call":
			var params struct {
				Arguments map[string]interface{} `json:"arguments"`
			}
			_ = json.Unmarshal(msg.Params, &params)
			text, _ := params.Arguments["text"].(string)
			result = map[string]interface{}{"content": []map[string]interface{}{{"type": "text", "text": "eco:" + text}}}
		default:
			result = map[string]interface{}{}
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": msg.ID, "result": result})
	}))
	defer server.Close()

	tool := mcpInvokerTool(store.MCPTool{Name: "echo", Transport: "http", URL: server.URL, ServerNome: "fake"})
	out := tool.invoke(context.Background(), map[string]interface{}{"text": "oi"})
	if out != "eco:oi" {
		t.Fatalf("resultado MCP = %q, esperado %q", out, "eco:oi")
	}
}
