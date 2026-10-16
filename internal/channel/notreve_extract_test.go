package channel

import "testing"

func TestExtractTextButtonsResponse(t *testing.T) {
	m := &notreveMessage{ButtonsResponseMessage: &struct {
		SelectedButtonID    string `json:"selectedButtonID"`
		SelectedDisplayText string `json:"selectedDisplayText"`
	}{SelectedButtonID: "4", SelectedDisplayText: "4"}}
	if got := extractText(m); got != "4" {
		t.Fatalf("esperava a nota do botão, obtive %q", got)
	}
}

func TestExtractTextListResponse(t *testing.T) {
	m := &notreveMessage{ListResponseMessage: &struct {
		Title           string `json:"title"`
		SingleSelectReply *struct {
			SelectedRowID string `json:"selectedRowID"`
		} `json:"singleSelectReply"`
	}{SingleSelectReply: &struct {
		SelectedRowID string `json:"selectedRowID"`
	}{SelectedRowID: "suporte"}}}
	if got := extractText(m); got != "suporte" {
		t.Fatalf("esperava o item selecionado, obtive %q", got)
	}
}

func TestExtractTextNativeFlowInteractive(t *testing.T) {
	m := &notreveMessage{InteractiveResponseMessage: &struct {
		Body *struct {
			Text string `json:"text"`
		} `json:"body"`
		NativeFlowResponseMessage *struct {
			Name       string `json:"name"`
			ParamsJSON string `json:"paramsJson"`
		} `json:"nativeFlowResponseMessage"`
	}{NativeFlowResponseMessage: &struct {
		Name       string `json:"name"`
		ParamsJSON string `json:"paramsJson"`
	}{Name: "quick_reply", ParamsJSON: `{"id":"4","display_text":"4"}`}}}
	if got := extractText(m); got != "4" {
		t.Fatalf("esperava a resposta do fluxo nativo, obtive %q", got)
	}
}
