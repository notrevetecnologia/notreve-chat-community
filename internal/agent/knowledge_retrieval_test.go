package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
)

type fakeKnowledgeSearcher struct {
	sources   []store.KnowledgeSource
	chunks    []store.KnowledgeChunk
	searchErr error
	hasChunks bool

	searchCalls int
	lastQuery   string
	lastLimit   int
}

func (f *fakeKnowledgeSearcher) ListKnowledgeSources(context.Context, int) ([]store.KnowledgeSource, error) {
	return f.sources, nil
}

func (f *fakeKnowledgeSearcher) SearchKnowledgeChunks(_ context.Context, _ int, query string, limit int) ([]store.KnowledgeChunk, error) {
	f.searchCalls++
	f.lastQuery = query
	f.lastLimit = limit
	return f.chunks, f.searchErr
}

func (f *fakeKnowledgeSearcher) HasKnowledgeChunks(context.Context, int) (bool, error) {
	return f.hasChunks, nil
}

func largeSource(name, filler string) store.KnowledgeSource {
	return store.KnowledgeSource{
		ID: 1, AgentID: 1, Name: name, Status: "ready",
		Content: filler + strings.Repeat(" conteúdo irrelevante", 1000),
	}
}

func TestKnowledgePromptForUsesRetrieval(t *testing.T) {
	searcher := &fakeKnowledgeSearcher{
		sources: []store.KnowledgeSource{largeSource("Manual", "início")},
		chunks: []store.KnowledgeChunk{
			{SourceID: 1, SourceName: "Manual", Content: "O plano Pro custa R$ 99 por mês.", Rank: 0.5},
		},
		hasChunks: true,
	}
	got := knowledgePromptFor(context.Background(), searcher, 1, "quanto custa o plano pro?", 6)

	if searcher.searchCalls != 1 {
		t.Fatalf("esperava 1 busca, obtive %d", searcher.searchCalls)
	}
	if searcher.lastQuery != "quanto custa o plano pro?" || searcher.lastLimit != 6 {
		t.Fatalf("busca chamada com query=%q limit=%d", searcher.lastQuery, searcher.lastLimit)
	}
	if !strings.Contains(got, "R$ 99") {
		t.Fatalf("retrieval não injetou o trecho relevante: %q", got)
	}
	if strings.Contains(got, "conteúdo irrelevante") {
		t.Fatalf("retrieval não deveria despejar a base inteira: %q", got)
	}
}

func TestKnowledgePromptForFallsBackOnSearchError(t *testing.T) {
	source := largeSource("Manual", "conteúdo")
	searcher := &fakeKnowledgeSearcher{sources: []store.KnowledgeSource{source}, searchErr: errors.New("boom")}
	got := knowledgePromptFor(context.Background(), searcher, 1, "alguma dúvida", 6)

	if got != fullKnowledgeBlock(searcher.sources) {
		t.Fatalf("falha na busca deveria cair no dump completo")
	}
	if !strings.Contains(got, "conteúdo irrelevante") {
		t.Fatalf("fallback não incluiu a base: %q", got)
	}
}

func TestKnowledgePromptForFallsBackOnSmallBase(t *testing.T) {
	source := store.KnowledgeSource{ID: 1, Name: "FAQ", Status: "ready", Content: "Pequena base de exemplo."}
	searcher := &fakeKnowledgeSearcher{
		sources: []store.KnowledgeSource{source},
		chunks:  []store.KnowledgeChunk{{SourceID: 1, SourceName: "FAQ", Content: "não usado"}},
	}
	got := knowledgePromptFor(context.Background(), searcher, 1, "alguma dúvida", 6)

	if searcher.searchCalls != 0 {
		t.Fatalf("base pequena não deveria consultar o retrieval")
	}
	if got != fullKnowledgeBlock(searcher.sources) {
		t.Fatalf("base pequena deveria usar o dump completo: %q", got)
	}
}

func TestKnowledgePromptForEmptyQueryUsesFull(t *testing.T) {
	searcher := &fakeKnowledgeSearcher{sources: []store.KnowledgeSource{largeSource("Manual", "conteúdo")}}
	got := knowledgePromptFor(context.Background(), searcher, 1, "   ", 6)

	if searcher.searchCalls != 0 {
		t.Fatalf("consulta vazia não deveria consultar o retrieval")
	}
	if got != fullKnowledgeBlock(searcher.sources) {
		t.Fatalf("consulta vazia deveria usar o dump completo")
	}
}

func TestKnowledgePromptForEmptyIndexFallsBack(t *testing.T) {
	searcher := &fakeKnowledgeSearcher{
		sources:   []store.KnowledgeSource{largeSource("Manual", "conteúdo")},
		hasChunks: false,
	}
	got := knowledgePromptFor(context.Background(), searcher, 1, "dúvida real", 6)

	if got != fullKnowledgeBlock(searcher.sources) {
		t.Fatalf("índice vazio deveria cair no dump completo: %q", got)
	}
}

func TestKnowledgePromptForIndexedNoMatchInjectsNothing(t *testing.T) {
	searcher := &fakeKnowledgeSearcher{
		sources:   []store.KnowledgeSource{largeSource("Manual", "conteúdo")},
		hasChunks: true,
	}
	got := knowledgePromptFor(context.Background(), searcher, 1, "assunto totalmente fora", 6)

	if got != "" {
		t.Fatalf("base indexada sem trecho relevante não deveria injetar nada: %q", got)
	}
}

func TestClampKnowledgeTopK(t *testing.T) {
	cases := map[int]int{0: defaultKnowledgeTopK, -3: defaultKnowledgeTopK, 1: 1, 8: 8, 50: 20}
	for input, want := range cases {
		if got := clampKnowledgeTopK(input); got != want {
			t.Fatalf("clampKnowledgeTopK(%d) = %d, esperava %d", input, got, want)
		}
	}
}
