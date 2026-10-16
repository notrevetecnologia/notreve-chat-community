package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/notrevetecnologia/notreve-chat-community/internal/llm"
	"github.com/notrevetecnologia/notreve-chat-community/internal/store"
	"github.com/notrevetecnologia/notreve-chat-community/internal/tools"
)

const (
	cfStepStart        = "start"
	cfStepMessage      = "message"
	cfStepCollect      = "collect"
	cfStepCondition    = "condition"
	cfStepIntent       = "intent"
	cfStepTool         = "tool"
	cfStepQueue        = "queue"
	cfStepEnd          = "end"
	cfStepMenu         = "menu"
	cfStepDelay        = "delay"
	cfStepSetVar       = "set_variable"
	cfStepGoto         = "goto"
	cfStepBusiness     = "business_hours"
	cfStepTag          = "tag"
	cfStepSatisfaction = "satisfaction"
	cfStepNote         = "note"
	cfStepHTTP         = "http"
	maxChatflowSteps   = 24
	maxChatflowJumps   = 24
	cfMaxDelaySeconds  = 30
)

type cfMessageConfig struct {
	Text     string          `json:"text"`
	Next     *int            `json:"next"`
	Type     string          `json:"type"` // buttons | list | poll
	Title    string          `json:"title"`
	Footer   string          `json:"footer"`
	Options  []string        `json:"options"` // botões/enquete simples
	Sections []cfListSection `json:"sections"`
	Rows     []cfListRow     `json:"rows"`
}

type cfListSection struct {
	Title string      `json:"title"`
	Rows  []cfListRow `json:"rows"`
}

type cfListRow struct {
	Title       string `json:"title"`
	Description string `json:"desc"`
	ID          string `json:"id"`
}

type cfCollectConfig struct {
	Prompt   string `json:"prompt"`
	Variable string `json:"variable"`
	Next     *int   `json:"next"`
}

type cfConditionConfig struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Then     *int   `json:"then"`
	Else     *int   `json:"else"`
}

type cfIntentBranch struct {
	Label string `json:"label"`
	Next  *int   `json:"next"`
}

type cfIntentConfig struct {
	Prompt   string           `json:"prompt"`
	Branches []cfIntentBranch `json:"branches"`
	Default  *int             `json:"default"`
}

type cfToolConfig struct {
	Name   string            `json:"name"`
	Inputs map[string]string `json:"inputs"`
	ToVar  string            `json:"to_var"` // guarda o resultado da tool na variável
	Next   *int              `json:"next"`
}

type cfTransferConfig struct {
	SetorID *int   `json:"setor_id"`
	Setor   string `json:"setor"`
	Text    string `json:"text"`
}

type cfStartConfig struct {
	Next *int `json:"next"`
}

type cfEndConfig struct {
	Text string `json:"text"`
}

type cfMenuOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Next  *int   `json:"next"`
}

type cfMenuConfig struct {
	Text    string         `json:"text"`
	Type    string         `json:"type"`
	Title   string         `json:"title"`
	Footer  string         `json:"footer"`
	Options []cfMenuOption `json:"options"`
	Default *int           `json:"default"`
}

type cfDelayConfig struct {
	Seconds int  `json:"seconds"`
	Minutes int  `json:"minutes"`
	Next    *int `json:"next"`
}

type cfSetVarConfig struct {
	Variable  string `json:"variable"`
	Value     string `json:"value"`
	Operation string `json:"operation"`
	Next      *int   `json:"next"`
}

type cfGotoConfig struct {
	Target *int `json:"target"`
}

type cfBusinessHoursConfig struct {
	Setor      string `json:"setor"`
	Abertura   string `json:"abertura"`
	Fechamento string `json:"fechamento"`
	Dias       string `json:"dias"`
	Open       *int   `json:"open"`
	Closed     *int   `json:"closed"`
}

type cfTagConfig struct {
	Tags string `json:"tags"`
	Next *int   `json:"next"`
}

type cfSatisfactionConfig struct {
	Text string `json:"text"`
	Next *int   `json:"next"`
	Skip *int   `json:"skip"`
}

type cfNoteConfig struct {
	Text string `json:"text"`
	Next *int   `json:"next"`
}

type cfHTTPConfig struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Inputs  map[string]string `json:"inputs"`
	ToVar   string            `json:"to_var"`
	Next    *int              `json:"next"`
}

type cfStep struct {
	ord  int
	tipo string
	cfg  []byte
}

type cfRun struct {
	e         *Engine
	ctx       context.Context
	convID    int
	flowID    int
	agent     store.Agent
	message   string
	vars      map[string]string
	order     []int
	byOrd     map[int]cfStep
	usedTools []string
	reply     Reply
	path      []int
	paused    bool
	jumps     int
	satSent   bool
}

type chatflowResolver interface {
	GetConversation(ctx context.Context, id int) (store.Conversation, error)
	ResolveActiveChatflow(ctx context.Context, chatflowID *int) (*store.Chatflow, []store.ChatflowStep, error)
}

type ChatflowTarget struct {
	Mode       string
	ChatflowID *int
}

func resolveChatflowFor(ctx context.Context, resolver chatflowResolver, convID int, target ChatflowTarget) (*store.Chatflow, []store.ChatflowStep, error) {
	conv, err := resolver.GetConversation(ctx, convID)
	if err != nil {
		return nil, nil, err
	}
	if conv.Status != "ia" {
		return nil, nil, nil
	}
	if target.Mode != "chatflow" {
		return nil, nil, nil
	}
	return resolver.ResolveActiveChatflow(ctx, target.ChatflowID)
}

func (e *Engine) resolveChatflow(ctx context.Context, convID int, target ChatflowTarget) (*store.Chatflow, []store.ChatflowStep, error) {
	return resolveChatflowFor(ctx, e.store, convID, target)
}

func (e *Engine) runChatflow(ctx context.Context, convID int, agent store.Agent, flow *store.Chatflow, steps []store.ChatflowStep, message string) (Reply, error) {
	state, err := e.store.GetConversationChatflowState(ctx, convID)
	if err != nil {
		return e.fallbackReply(ctx), nil
	}
	order := make([]int, 0, len(steps))
	byOrd := make(map[int]cfStep, len(steps))
	for _, step := range steps {
		if _, dup := byOrd[step.Ord]; dup {
			continue
		}
		order = append(order, step.Ord)
		byOrd[step.Ord] = cfStep{ord: step.Ord, tipo: step.Tipo, cfg: []byte(step.Config)}
	}
	sort.Ints(order)
	if len(order) == 0 {
		return e.fallbackReply(ctx), nil
	}
	if state.Vars == nil {
		state.Vars = map[string]string{}
	}
	r := &cfRun{
		e: e, ctx: ctx, convID: convID, flowID: flow.ID, agent: agent, message: message,
		vars: state.Vars, order: order, byOrd: byOrd, path: []int{},
	}

	start := cfStartOrd(order, byOrd)
	if state.StepOrd != nil {
		if step, ok := byOrd[*state.StepOrd]; ok {
			switch step.tipo {
			case cfStepCollect:
				var cfg cfCollectConfig
				if json.Unmarshal(step.cfg, &cfg) == nil && cfg.Variable != "" {
					r.vars[cfg.Variable] = strings.TrimSpace(message)
				}
				if next, ok := r.nextAfter(*state.StepOrd, cfg.Next); ok {
					start = next
				}
			case cfStepMenu:
				var cfg cfMenuConfig
				if json.Unmarshal(step.cfg, &cfg) == nil {
					start = r.resolveMenuResume(*state.StepOrd, cfg, message)
				}
			default:
				start = *state.StepOrd
			}
		}
	}

	if err := r.advance(start); err != nil {
		return e.fallbackReply(ctx), nil
	}
	r.reply.UsedTools = r.usedTools

	if !r.paused && !r.reply.NeedsHuman {
		_ = r.e.store.ClearConversationChatflow(r.ctx, r.convID)
		if r.reply.Text == "" {
			r.reply.Text = "Como posso ajudar?"
		}
	}
	return r.reply, nil
}

func (r *cfRun) advance(ord int) error {
	if len(r.path) >= maxChatflowSteps {
		return nil
	}
	if step, ok := r.byOrd[ord]; ok {
		if containsInt(r.path, ord) {
			return nil
		}
		r.path = append(r.path, ord)
		return r.execute(step)
	}
	return nil
}

func (r *cfRun) nextAfter(ord int, explicit *int) (int, bool) {
	return cfNextAfter(r.order, ord, explicit)
}

func (r *cfRun) execute(step cfStep) error {
	switch step.tipo {
	case cfStepMessage:
		var cfg cfMessageConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		text := interpolateVars(cfg.Text, r.vars)
		r.appendText(text)
		if rich := cfRichMessage(cfg, text); rich != nil {
			r.reply.Rich = rich
			r.reply.Fallback = rich.Fallback
		}
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepCollect:
		var cfg cfCollectConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.appendText(interpolateVars(cfg.Prompt, r.vars))
		ord := step.ord
		r.paused = true
		if err := r.e.store.SetConversationChatflowState(r.ctx, r.convID, r.flowID, &ord, r.vars); err != nil {
			return err
		}
		return nil
	case cfStepCondition:
		var cfg cfConditionConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		branch := cfg.Then
		if !r.evalCondition(cfg) {
			branch = cfg.Else
		}
		if next, ok := r.nextAfter(step.ord, branch); ok {
			return r.advance(next)
		}
		return nil
	case cfStepIntent:
		var cfg cfIntentConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if next, ok := r.resolveIntent(cfg); ok {
			return r.advance(next)
		}
		if next, ok := r.nextAfter(step.ord, cfg.Default); ok {
			return r.advance(next)
		}
		return nil
	case cfStepTool:
		var cfg cfToolConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.executeTool(cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepQueue:
		var cfg cfTransferConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.enqueueSetor(cfg)
		if r.reply.Text == "" {
			r.reply.Text = "Você foi direcionado para o setor de atendimento. Em breve um atendente vai te responder. 😊"
		}
		return nil
	case cfStepStart:
		var cfg cfStartConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepEnd:
		var cfg cfEndConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.appendText(cfg.Text)
		if r.reply.Text == "" {
			r.reply.Text = "Obrigado pelo contato! Até logo. 😊"
		}
		if !r.satSent {
			_ = r.e.store.EncerrarConversationSilencioso(r.ctx, r.convID, "Concluído pelo chat flow de atendimento", nil)
		}
		_ = r.e.store.ClearConversationChatflow(r.ctx, r.convID)
		return nil
	case cfStepMenu:
		var cfg cfMenuConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		body := interpolateVars(cfg.Text, r.vars)
		r.appendText(cfMenuPrompt(body, cfg.Options))
		if rich := cfMenuRich(cfg, body); rich != nil {
			r.reply.Rich = rich
			r.reply.Fallback = rich.Fallback
		}
		ord := step.ord
		r.paused = true
		if err := r.e.store.SetConversationChatflowState(r.ctx, r.convID, r.flowID, &ord, r.vars); err != nil {
			return err
		}
		return nil
	case cfStepDelay:
		var cfg cfDelayConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if wait := cfDelaySeconds(cfg); wait > 0 {
			timer := time.NewTimer(time.Duration(wait) * time.Second)
			select {
			case <-r.ctx.Done():
				timer.Stop()
				return r.ctx.Err()
			case <-timer.C:
			}
		}
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepSetVar:
		var cfg cfSetVarConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.applySetVar(cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepGoto:
		var cfg cfGotoConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if cfg.Target == nil {
			if next, ok := r.nextAfter(step.ord, nil); ok {
				return r.advance(next)
			}
			return nil
		}
		return r.jumpTo(*cfg.Target)
	case cfStepBusiness:
		var cfg cfBusinessHoursConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		branch := cfg.Closed
		if r.businessHoursOpen(cfg) {
			branch = cfg.Open
		}
		if next, ok := r.nextAfter(step.ord, branch); ok {
			return r.advance(next)
		}
		return nil
	case cfStepTag:
		var cfg cfTagConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.applyTags(cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepSatisfaction:
		var cfg cfSatisfactionConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if r.dispatchSatisfaction(cfg) {
			if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
				return r.advance(next)
			}
			return nil
		}
		if next, ok := r.nextAfter(step.ord, cfg.Skip); ok {
			return r.advance(next)
		}
		return nil
	case cfStepNote:
		var cfg cfNoteConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	case cfStepHTTP:
		var cfg cfHTTPConfig
		_ = json.Unmarshal(step.cfg, &cfg)
		r.executeHTTP(cfg)
		if next, ok := r.nextAfter(step.ord, cfg.Next); ok {
			return r.advance(next)
		}
		return nil
	default:
		if next, ok := r.nextAfter(step.ord, nil); ok {
			return r.advance(next)
		}
		return nil
	}
}

func (r *cfRun) appendText(text string) {
	cfAppendText(&r.reply, text)
}

func (r *cfRun) enqueueSetor(cfg cfTransferConfig) {
	r.reply.NeedsHuman = true
	if cfg.Setor != "" {
		if setor, err := r.e.store.FindSetorByName(r.ctx, cfg.Setor); err == nil && setorPermitido(setor.ID, r.agent.SetorIDs) {
			sid := setor.ID
			r.reply.SetorID = &sid
		}
	} else if cfg.SetorID != nil && setorPermitido(*cfg.SetorID, r.agent.SetorIDs) {
		sid := *cfg.SetorID
		r.reply.SetorID = &sid
	}
	r.appendText(interpolateVars(cfg.Text, r.vars))
	_ = r.e.store.ClearConversationChatflow(r.ctx, r.convID)
}

func (r *cfRun) resolveMenuResume(ord int, cfg cfMenuConfig, message string) int {
	if index, ok := cfMenuMatch(cfg, message); ok {
		if next, ok := r.nextAfter(ord, cfg.Options[index].Next); ok {
			return next
		}
		return ord
	}
	if cfg.Default != nil {
		if next, ok := r.nextAfter(ord, cfg.Default); ok {
			return next
		}
	}
	return ord
}

func (r *cfRun) jumpTo(ord int) error {
	if r.jumps >= maxChatflowJumps {
		return nil
	}
	if _, ok := r.byOrd[ord]; !ok {
		return nil
	}
	r.jumps++
	r.path = nil
	return r.advance(ord)
}

func (r *cfRun) businessHoursOpen(cfg cfBusinessHoursConfig) bool {
	if strings.TrimSpace(cfg.Setor) != "" {
		if setor, err := r.e.store.FindSetorByName(r.ctx, cfg.Setor); err == nil {
			return setorAberto(setor, brazilNow())
		}
	}
	return cfBusinessHoursOpen(cfg, brazilNow())
}

func (r *cfRun) applySetVar(cfg cfSetVarConfig) {
	name := strings.TrimSpace(cfg.Variable)
	if name == "" {
		return
	}
	value := interpolateVars(cfg.Value, r.vars)
	switch strings.ToLower(strings.TrimSpace(cfg.Operation)) {
	case "append", "concat", "anexar":
		r.vars[name] += value
	case "prepend", "prefixar":
		r.vars[name] = value + r.vars[name]
	case "add", "sum", "increment", "somar":
		current, _ := strconv.Atoi(strings.TrimSpace(r.vars[name]))
		delta, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil {
			r.vars[name] = strconv.Itoa(current + delta)
		}
	default:
		r.vars[name] = value
	}
}

func (r *cfRun) applyTags(cfg cfTagConfig) {
	raw := interpolateVars(cfg.Tags, r.vars)
	var tags []string
	for _, part := range strings.Split(raw, ",") {
		if tag := strings.TrimSpace(part); tag != "" {
			tags = append(tags, tag)
		}
	}
	if len(tags) == 0 {
		return
	}
	conv, err := r.e.store.GetConversation(r.ctx, r.convID)
	if err != nil || strings.TrimSpace(conv.CustomerPhone) == "" {
		return
	}
	contato, err := r.e.store.GetContatoByPhone(r.ctx, conv.CustomerPhone)
	if err != nil {
		_, _ = r.e.store.CreateContato(r.ctx, store.Contato{Phone: conv.CustomerPhone, Tags: strings.Join(tags, ",")})
		return
	}
	seen := map[string]bool{}
	merged := make([]string, 0, len(tags))
	for _, part := range strings.Split(contato.Tags, ",") {
		tag := strings.TrimSpace(part)
		key := strings.ToLower(tag)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, tag)
	}
	for _, tag := range tags {
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, tag)
	}
	contato.Tags = strings.Join(merged, ",")
	_ = r.e.store.UpdateContato(r.ctx, contato)
}

func (r *cfRun) dispatchSatisfaction(cfg cfSatisfactionConfig) bool {
	hasAI, hasHuman, err := r.e.store.SatisfactionParticipants(r.ctx, r.convID)
	if err != nil || (!hasAI && !hasHuman) {
		return false
	}
	target := "ia"
	if !hasAI && hasHuman {
		target = "operador"
	}
	text := cfSatisfactionQuestion(r.ctx, r.e.store, target)
	if custom := strings.TrimSpace(interpolateVars(cfg.Text, r.vars)); custom != "" {
		text = custom
	}
	if _, err := r.e.store.EncerrarConversationIfOpen(r.ctx, r.convID, "Pesquisa de satisfação (chat flow)", nil); err != nil {
		return false
	}
	if err := r.e.store.SetConversationSatisfacao(r.ctx, r.convID, "aguardando"); err != nil {
		return false
	}
	r.satSent = true
	r.appendText(text)
	if rich := cfSatisfactionRich(text); rich != nil {
		r.reply.Rich = rich
		r.reply.Fallback = rich.Fallback
	}
	return true
}

func (r *cfRun) executeHTTP(cfg cfHTTPConfig) {
	rawURL := strings.TrimSpace(interpolateVars(cfg.URL, r.vars))
	if rawURL == "" {
		return
	}
	method := strings.ToUpper(strings.TrimSpace(cfg.Method))
	if method == "" {
		method = "GET"
	}
	headers := make(map[string]string, len(cfg.Headers))
	for key, value := range cfg.Headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		headers[key] = interpolateVars(value, r.vars)
	}
	args := map[string]interface{}{}
	for key, value := range cfg.Inputs {
		if strings.TrimSpace(key) == "" {
			continue
		}
		args[key] = interpolateVars(value, r.vars)
	}
	def := store.Tool{
		Name: "chatflow_http", Method: method, URL: rawURL, Headers: headers,
		Parameters: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
	result := strings.TrimSpace(r.e.exec.Execute(r.ctx, def, args))
	if result == "" {
		return
	}
	r.usedTools = append(r.usedTools, "chatflow_http")
	if strings.TrimSpace(cfg.ToVar) != "" {
		r.vars[cfg.ToVar] = result
		return
	}
	r.appendText(result)
}

func cfMenuPrompt(body string, options []cfMenuOption) string {
	var b strings.Builder
	if trimmed := strings.TrimSpace(body); trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n")
	}
	for i, opt := range options {
		label := strings.TrimSpace(opt.Label)
		if label == "" {
			continue
		}
		fmt.Fprintf(&b, "%d. %s\n", i+1, label)
	}
	out := strings.TrimRight(b.String(), "\n")
	if out == "" {
		return "Escolha uma opção:"
	}
	return out
}

func cfMenuRich(cfg cfMenuConfig, body string) *RichMessage {
	if len(cfg.Options) == 0 {
		return nil
	}
	switch cfg.Type {
	case "buttons":
		buttons := make([]OptionButton, 0, len(cfg.Options))
		for i, opt := range cfg.Options {
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				continue
			}
			buttons = append(buttons, OptionButton{Type: "reply", Title: label, ID: strconv.Itoa(i + 1)})
		}
		if len(buttons) == 0 {
			return nil
		}
		rich := &RichMessage{Kind: "buttons", Text: body, Buttons: buttons}
		rich.Fallback = rich.FallbackText()
		return rich
	case "list":
		rows := make([]ListRow, 0, len(cfg.Options))
		for i, opt := range cfg.Options {
			label := strings.TrimSpace(opt.Label)
			if label == "" {
				continue
			}
			rows = append(rows, ListRow{Title: label, Description: strings.TrimSpace(opt.Value), ID: strconv.Itoa(i + 1)})
		}
		if len(rows) == 0 {
			return nil
		}
		rich := &RichMessage{Kind: "list", Text: body, Title: cfg.Title, Footer: cfg.Footer, Sections: []ListSection{{Rows: rows}}}
		rich.Fallback = rich.FallbackText()
		return rich
	default:
		return nil
	}
}

func cfMenuMatch(cfg cfMenuConfig, message string) (int, bool) {
	text := strings.TrimSpace(message)
	if text == "" || len(cfg.Options) == 0 {
		return 0, false
	}
	if n, err := strconv.Atoi(text); err == nil && n >= 1 && n <= len(cfg.Options) {
		return n - 1, true
	}
	for i, opt := range cfg.Options {
		if label := strings.TrimSpace(opt.Label); label != "" && strings.EqualFold(label, text) {
			return i, true
		}
		if value := strings.TrimSpace(opt.Value); value != "" && strings.EqualFold(value, text) {
			return i, true
		}
	}
	return 0, false
}

func cfDelaySeconds(cfg cfDelayConfig) int {
	total := cfg.Seconds + cfg.Minutes*60
	if total <= 0 {
		return 0
	}
	if total > cfMaxDelaySeconds {
		return cfMaxDelaySeconds
	}
	return total
}

func cfBusinessHoursOpen(cfg cfBusinessHoursConfig, now time.Time) bool {
	abertura := strings.TrimSpace(cfg.Abertura)
	fechamento := strings.TrimSpace(cfg.Fechamento)
	if abertura == "" && fechamento == "" {
		return true
	}
	dias := strings.TrimSpace(cfg.Dias)
	if dias == "" {
		dias = "0,1,2,3,4,5,6"
	}
	return setorAberto(store.Setor{HorarioAbertura: abertura, HorarioFechamento: fechamento, DiasSemana: dias}, now)
}

func cfSatisfactionQuestion(ctx context.Context, st *store.Store, target string) string {
	key := "satisfacao_ia"
	text := "De 1 a 5, como você avalia o atendimento da nossa assistente virtual? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊"
	if target == "operador" {
		key = "satisfacao_operador"
		text = "De 1 a 5, como você avalia o atendimento do nosso atendente? Responda apenas com um número (1 = péssimo, 5 = excelente). 😊"
	}
	if message, err := st.GetMensagemByChave(ctx, key); err == nil && strings.TrimSpace(message.Texto) != "" {
		return strings.TrimSpace(message.Texto)
	}
	return text
}

func cfSatisfactionRich(text string) *RichMessage {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	rich := &RichMessage{Kind: "buttons", Text: text}
	for i := 1; i <= 5; i++ {
		label := strconv.Itoa(i)
		rich.Buttons = append(rich.Buttons, OptionButton{Type: "reply", Title: label, ID: label})
	}
	rich.Fallback = text
	return rich
}

// cfRichMessage monta uma mensagem rica (botões/lista/enquete) a partir da
// configuração de um passo message. Devolve nil se o tipo não for suportado ou
// os dados forem insuficientes — nesse caso o fluxo segue com o texto puro.
func cfRichMessage(cfg cfMessageConfig, body string) *RichMessage {
	switch cfg.Type {
	case "buttons":
		options := cfg.Options
		if len(options) == 0 {
			for _, row := range cfg.Rows {
				if row.Title != "" {
					options = append(options, row.Title)
				}
			}
		}
		rich := buildRichButtons(body, options)
		if rich == nil {
			return nil
		}
		rich.Fallback = rich.FallbackText()
		return rich
	case "list":
		sections := listSectionsFromRows(cfg.Sections, cfg.Rows)
		if len(sections) == 0 {
			return nil
		}
		rich := &RichMessage{Kind: "list", Text: body, Title: cfg.Title, Footer: cfg.Footer, Sections: sections}
		rich.Fallback = rich.FallbackText()
		return rich
	case "poll":
		options := cfg.Options
		if len(options) < 2 {
			return nil
		}
		pollOptions := make([]PollOption, 0, len(options))
		for _, opt := range options {
			opt = strings.TrimSpace(opt)
			if opt != "" {
				pollOptions = append(pollOptions, PollOption{Title: opt, ID: opt})
			}
		}
		if len(pollOptions) < 2 {
			return nil
		}
		rich := &RichMessage{Kind: "poll", Text: body, Poll: &Poll{Question: body, Options: pollOptions}}
		rich.Fallback = rich.FallbackText()
		return rich
	default:
		return nil
	}
}

func listSectionsFromRows(sections []cfListSection, flat []cfListRow) []ListSection {
	if len(sections) > 0 {
		out := make([]ListSection, 0, len(sections))
		for _, sec := range sections {
			if sec.Title == "" && len(sec.Rows) == 0 {
				continue
			}
			out = append(out, ListSection{Title: sec.Title, Rows: rowsFromCF(sec.Rows)})
		}
		return out
	}
	if len(flat) > 0 {
		return []ListSection{{Title: "", Rows: rowsFromCF(flat)}}
	}
	return nil
}

func rowsFromCF(rows []cfListRow) []ListRow {
	out := make([]ListRow, 0, len(rows))
	for _, row := range rows {
		if row.Title == "" {
			continue
		}
		id := row.ID
		if id == "" {
			id = row.Title
		}
		out = append(out, ListRow{Title: row.Title, Description: row.Description, ID: id})
	}
	return out
}

func (r *cfRun) evalCondition(cfg cfConditionConfig) bool {
	return cfEvalCondition(r.message, r.vars, cfg)
}

func cfStartOrd(order []int, byOrd map[int]cfStep) int {
	for _, ord := range order {
		if byOrd[ord].tipo == cfStepStart {
			return ord
		}
	}
	return order[0]
}

func cfNextAfter(order []int, ord int, explicit *int) (int, bool) {
	if explicit != nil {
		for _, candidate := range order {
			if candidate == *explicit {
				return candidate, true
			}
		}
	}
	idx := sort.SearchInts(order, ord)
	next := idx + 1
	if next < len(order) {
		return order[next], true
	}
	return 0, false
}

func cfAppendText(reply *Reply, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if reply.Text == "" {
		reply.Text = text
		return
	}
	if !strings.HasSuffix(reply.Text, "\n") {
		reply.Text += "\n"
	}
	reply.Text += text
}

func cfEvalCondition(message string, vars map[string]string, cfg cfConditionConfig) bool {
	target := ""
	if cfg.Field == "" || cfg.Field == "text" || cfg.Field == "mensagem" {
		target = strings.TrimSpace(message)
	} else {
		target = strings.TrimSpace(vars[cfg.Field])
	}
	value := strings.TrimSpace(cfg.Value)
	switch strings.ToLower(cfg.Operator) {
	case "eq", "equals", "=", "igual":
		return strings.EqualFold(target, value)
	case "contains", "contem", "contém", "inclui", "has":
		return strings.Contains(strings.ToLower(target), strings.ToLower(value))
	default:
		return strings.EqualFold(target, value)
	}
}

func (r *cfRun) resolveIntent(cfg cfIntentConfig) (int, bool) {
	if len(cfg.Branches) == 0 {
		return 0, false
	}
	pc, err := r.e.resolveProviderConfig(r.ctx, r.agent)
	if err != nil || pc.APIKey == "" {
		return r.intentFallback(cfg)
	}
	client := llm.NewClient(pc.BaseURL, pc.APIKey, pc.Model, pc.Temp)
	var b strings.Builder
	b.WriteString("Você roteia mensagens de atendimento para um dos ramos abaixo.\n")
	for i, branch := range cfg.Branches {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, branch.Label))
	}
	b.WriteString("Responda apenas com o número correspondente ao ramo mais adequado à mensagem do cliente.")
	system := b.String()
	if cfg.Prompt != "" {
		system = cfg.Prompt
	}
	resp, err := client.Complete(r.ctx, []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: r.message},
	}, nil)
	if err != nil {
		return r.intentFallback(cfg)
	}
	out := strings.TrimSpace(contentString(resp.Choices[0].Message.Content))
	num := extractNota(out)
	if num < 1 || num > len(cfg.Branches) {
		return r.intentFallback(cfg)
	}
	if n, ok := r.validBranchOrd(cfg.Branches[num-1].Next); ok {
		return n, true
	}
	return r.intentFallback(cfg)
}

func (r *cfRun) intentFallback(cfg cfIntentConfig) (int, bool) {
	if n, ok := r.validBranchOrd(cfg.Default); ok {
		return n, true
	}
	for _, branch := range cfg.Branches {
		if n, ok := r.validBranchOrd(branch.Next); ok {
			return n, true
		}
	}
	return 0, false
}

func (r *cfRun) validBranchOrd(next *int) (int, bool) {
	if next == nil || *next <= 0 {
		return 0, false
	}
	if _, ok := r.byOrd[*next]; ok {
		return *next, true
	}
	return 0, false
}

func (r *cfRun) executeTool(cfg cfToolConfig) {
	integrations, err := r.e.store.GetIntegrationsByIDs(r.ctx, r.agent.ToolIDs)
	if err != nil {
		return
	}
	def := findTool(integrations, cfg.Name)
	if def.Name == "" {
		return
	}
	args := map[string]interface{}{}
	for key, value := range cfg.Inputs {
		args[key] = interpolateVars(value, r.vars)
	}
	toolCtx := r.ctx
	if phone, err := r.e.store.GetConversation(r.ctx, r.convID); err == nil && phone.CustomerPhone != "" {
		toolCtx = tools.WithToolIdentityPhone(r.ctx, phone.CustomerPhone)
	}
	result := r.e.exec.Execute(toolCtx, def, args)
	result = strings.TrimSpace(result)
	if result == "" {
		return
	}
	r.usedTools = append(r.usedTools, def.Name)
	if strings.TrimSpace(cfg.ToVar) != "" {
		r.vars[cfg.ToVar] = result
		return
	}
	r.appendText(result)
}

// interpolateVars substitui tokens {{chave}} por valores das variáveis do fluxo.
func interpolateVars(text string, vars map[string]string) string {
	if !strings.Contains(text, "{{") {
		return text
	}
	var b strings.Builder
	for {
		start := strings.Index(text, "{{")
		if start < 0 {
			b.WriteString(text)
			break
		}
		end := strings.Index(text[start:], "}}")
		if end < 0 {
			b.WriteString(text)
			break
		}
		end += start + 2
		key := strings.TrimSpace(text[start+2 : end-2])
		b.WriteString(text[:start])
		b.WriteString(vars[key])
		text = text[end:]
	}
	return b.String()
}

func containsInt(list []int, value int) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
