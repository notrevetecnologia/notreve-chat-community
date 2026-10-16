package store

import (
	"encoding/json"
	"time"
)

type Settings struct {
	ID                         int       `json:"id"`
	TenantID                   int       `json:"tenant_id"`
	DefaultBaseURL             string    `json:"default_base_url"`
	DefaultModel               string    `json:"default_model"`
	DefaultAPIKey              string    `json:"default_api_key"`
	Temperature                float64   `json:"temperature"`
	WhatsAppBaseURL            string    `json:"whatsapp_base_url"`
	WhatsAppToken              string    `json:"whatsapp_token"`
	ModoAberto                 bool      `json:"modo_aberto"`
	IAMonitora                 bool      `json:"ia_monitora"`
	NumerosPermitidos          string    `json:"numeros_permitidos"`
	DistribuicaoAuto           bool      `json:"distribuicao_automatica"`
	FollowupAtivo              bool      `json:"followup_ativo"`
	FollowupHoras              int       `json:"followup_horas"`
	IaNaFila                   bool      `json:"ia_na_fila"`
	WhatsAppAdminToken         string    `json:"whatsapp_admin_token"`
	WebhookBaseURL             string    `json:"webhook_base_url"`
	STTBaseURL                 string    `json:"stt_base_url"`
	STTAPIKey                  string    `json:"stt_api_key"`
	STTModel                   string    `json:"stt_model"`
	BrandName                  string    `json:"brand_name"`
	LogoURL                    string    `json:"logo_url"`
	FaviconURL                 string    `json:"favicon_url"`
	PrimaryColor               string    `json:"primary_color"`
	SecondaryColor             string    `json:"secondary_color"`
	AccentColor                string    `json:"accent_color"`
	BackgroundColor            string    `json:"background_color"`
	LoadingColor               string    `json:"loading_color"`
	DetectarBots               bool      `json:"detectar_bots"`
	BotPatterns                string    `json:"bot_patterns"`
	RegistrarFalhasAprendizado bool      `json:"registrar_falhas_aprendizado"`
	TransferirFalhasHumano     bool      `json:"transferir_falhas_humano"`
	KnowledgeTopK              int       `json:"knowledge_top_k"`
	WatchdogAtivo              bool      `json:"watchdog_ativo"`
	WatchdogMinutos            int       `json:"watchdog_minutos"`
	MediaTTLDias               int       `json:"media_ttl_dias"`
	MediaPurgaAtivo            bool      `json:"media_purga_ativo"`
	ImageModel                 string    `json:"image_model"`
	ImageAspectRatio           string    `json:"image_aspect_ratio"`
	LogsRetencaoDias           int       `json:"logs_retencao_dias"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type Tool struct {
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	Method                string                 `json:"method"`
	URL                   string                 `json:"url"`
	Headers               map[string]string      `json:"headers"`
	Parameters            map[string]interface{} `json:"parameters"`
	Connector             string                 `json:"connector,omitempty"`
	Risk                  string                 `json:"risk,omitempty"`
	RequiresConfirmation  bool                   `json:"requires_confirmation,omitempty"`
	RequiredScopes        []string               `json:"required_scopes,omitempty"`
	AllowedHosts          []string               `json:"allowed_hosts,omitempty"`
	TimeoutSeconds        int                    `json:"timeout_seconds,omitempty"`
	MaxCallsPerMinute     int                    `json:"max_calls_per_minute,omitempty"`
	MaxCallsPerHour       int                    `json:"max_calls_per_hour,omitempty"`
	MaxResponseBytes      int                    `json:"max_response_bytes,omitempty"`
	MaxArgumentBytes      int                    `json:"max_argument_bytes,omitempty"`
	RequiresIdentity      string                 `json:"requires_identity,omitempty"`
	AllowedIdentities     []string               `json:"allowed_identities,omitempty"`
	IdentityConfig        string                 `json:"identity_config,omitempty"`
	IdentityVerifyURL     string                 `json:"identity_verify_url,omitempty"`
	IdentityVerifyMethod  string                 `json:"identity_verify_method,omitempty"`
	IdentityVerifyParam   string                 `json:"identity_verify_param,omitempty"`
	IdentityMatchField    string                 `json:"identity_match_field,omitempty"`
	IdentityClientIDField string                 `json:"identity_client_id_field,omitempty"`
}

type Usuario struct {
	ID           int       `json:"id"`
	TenantID     int       `json:"tenant_id"`
	Name         string    `json:"name"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // super_admin | admin | supervisor | atendente
	Permissions  string    `json:"permissions"`
	AtendenteID  *int      `json:"atendente_id"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
}

type Module struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Sort        int    `json:"sort"`
}

type RespostaRapida struct {
	ID        int       `json:"id"`
	Atalho    string    `json:"atalho"`
	Texto     string    `json:"texto"`
	UserID    int       `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Contato struct {
	ID           int               `json:"id"`
	Name         string            `json:"name"`
	Phone        string            `json:"phone"`
	Email        string            `json:"email"`
	Notes        string            `json:"notes"`
	TemWhatsapp  bool              `json:"tem_whatsapp"`
	Tags         string            `json:"tags"`
	CamposExtras map[string]string `json:"campos_extras"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type Aprendizado struct {
	ID        int       `json:"id"`
	Pergunta  string    `json:"pergunta"`
	Canal     string    `json:"canal"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AtendenteDisponivel é um candidato à distribuição automática.
type AtendenteDisponivel struct {
	UsuarioID         int      `json:"usuario_id"`
	Nome              string   `json:"nome"`
	AtendenteID       int      `json:"atendente_id"`
	HorarioAbertura   string   `json:"horario_abertura"`
	HorarioFechamento string   `json:"horario_fechamento"`
	DiasSemana        string   `json:"dias_semana"`
	Setores           []int    `json:"setores"`
	Habilidades       []string `json:"habilidades,omitempty"`
	Carga             int      `json:"carga"`
	Capacidade        int      `json:"capacidade"`
}

type Setor struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	HorarioAbertura   string    `json:"horario_abertura"`
	HorarioFechamento string    `json:"horario_fechamento"`
	DiasSemana        string    `json:"dias_semana"`
	Cor               string    `json:"cor"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
}

type Atendente struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Active            bool      `json:"active"`
	Online            bool      `json:"online"`
	Username          string    `json:"username,omitempty"`
	Role              string    `json:"role,omitempty"`
	Permissions       string    `json:"permissions,omitempty"`
	HorarioAbertura   string    `json:"horario_abertura"`
	HorarioFechamento string    `json:"horario_fechamento"`
	DiasSemana        string    `json:"dias_semana"`
	SetorIDs          []int     `json:"setor_ids"`
	Habilidades       []string  `json:"habilidades,omitempty"`
	Capacidade        int       `json:"capacidade"`
	CreatedAt         time.Time `json:"created_at"`
}

type Mensagem struct {
	ID        int       `json:"id"`
	Chave     string    `json:"chave"`
	Titulo    string    `json:"titulo"`
	Texto     string    `json:"texto"`
	CreatedAt time.Time `json:"created_at"`
}

type Provider struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	ProviderType string    `json:"provider_type"` // notreve | openai_compat
	BaseURL      string    `json:"base_url"`
	Model        string    `json:"model"`
	APIKey       string    `json:"api_key"`
	Active       bool      `json:"active"`
	SafeMode     bool      `json:"safe_mode"`
	Capacidade   string    `json:"capacidade"` // texto | multimodal
	Capabilities []string  `json:"capabilities"`
	CreatedAt    time.Time `json:"created_at"`
}

type Channel struct {
	ID                int        `json:"id"`
	TenantID          int        `json:"-"`
	Type              string     `json:"type"` // whatsapp | webchat | instagram | telegram
	Name              string     `json:"name"`
	BaseURL           string     `json:"base_url"`
	Token             string     `json:"token"`
	InstanceID        string     `json:"instance_id,omitempty"`
	Active            bool       `json:"active"`
	RespondGrupos     bool       `json:"respond_grupos"`
	ResponderChamadas bool       `json:"responder_chamadas"`
	CanCampaign       bool       `json:"can_campaign"`
	WidgetConfig      string     `json:"widget_config"`
	PublicID          string     `json:"public_id"`
	HealthStatus      string     `json:"health_status"`
	LastCheckedAt     *time.Time `json:"last_checked_at,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
	AgentID           *int       `json:"agent_id,omitempty"`
	Mode              string     `json:"mode"`
	ChatflowID        *int       `json:"chatflow_id,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
}

type Segmento struct {
	ID                int       `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	FilterTemWhatsapp *bool     `json:"filter_tem_whatsapp,omitempty"`
	FilterCanal       string    `json:"filter_canal,omitempty"`
	Count             int       `json:"count,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type GrupoCanal struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	ChannelIDs   []int     `json:"channel_ids"`
	ChannelNames []string  `json:"channel_names"`
	CreatedAt    time.Time `json:"created_at"`
}

type Campanha struct {
	ID                int        `json:"id"`
	Name              string     `json:"name"`
	Message           string     `json:"message"`
	Canal             string     `json:"canal"`
	GrupoID           *int       `json:"grupo_id"`
	GrupoName         string     `json:"grupo_name"`
	ChannelID         *int       `json:"channel_id"`
	ChannelName       string     `json:"channel_name"`
	SegmentoID        *int       `json:"segmento_id"`
	SegmentoName      string     `json:"segmento_name"`
	Status            string     `json:"status"`
	ScheduledAt       *time.Time `json:"scheduled_at"`
	IntervaloSegundos int        `json:"intervalo_segundos"`
	LimiteDiario      int        `json:"limite_diario"`
	VariarMensagem    bool       `json:"variar_mensagem"`
	ModoTeste         bool       `json:"modo_teste"`
	MediaType         string     `json:"media_type"`
	MediaB64          string     `json:"media_b64"`
	MediaMime         string     `json:"media_mime"`
	MediaName         string     `json:"media_name"`
	Total             int        `json:"total"`
	Enviadas          int        `json:"enviadas"`
	Falhas            int        `json:"falhas"`
	Lidas             int        `json:"lidas"`
	CriadoPor         *int       `json:"criado_por"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	StartedAt         *time.Time `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

type CampanhaEnvio struct {
	ID          int        `json:"id"`
	CampanhaID  int        `json:"campanha_id"`
	ContatoID   int        `json:"contato_id"`
	Phone       string     `json:"phone"`
	ChannelID   *int       `json:"channel_id"`
	ChannelName string     `json:"channel_name"`
	Status      string     `json:"status"`
	Erro        string     `json:"erro"`
	WAMsgID     string     `json:"wa_msg_id"`
	LidaAt      *time.Time `json:"lida_at"`
	CreatedAt   time.Time  `json:"created_at"`
	SentAt      *time.Time `json:"sent_at"`
}

// MediaTypeOrText descreve o conteúdo da campanha para logs/UI.
func (c Campanha) MediaTypeOrText() string {
	if c.MediaType != "" {
		return "midia " + c.MediaType
	}
	return "texto"
}

type RelatorioResumo struct {
	Conversas     int
	Ativas        int
	Aguardando    int
	Encerradas    int
	Mensagens     int
	MsgCliente    int
	MsgIA         int
	MsgHumano     int
	ContatosNovos int
	SatGeral      float64
	SatIA         float64
	SatOp         float64
	Avaliacoes    int
	AvaliacoesIA  int
	AvaliacoesOp  int
	Campanhas     int
	CampTotal     int
	CampEnviadas  int
	CampFalhas    int
}

type RelAtendente struct {
	Nome         string
	Conversas    int
	MsgHumanas   int
	Avaliacoes   int
	NotaOperador float64
}

type RelSetor struct {
	Nome       string
	Conversas  int
	Avaliacoes int
	NotaGeral  float64
	NotaIA     float64
	NotaHumano float64
}

type RelAvaliacaoAtendimento struct {
	ConversationID int
	Atendente      string
	Setor          string
	Canal          string
	NotaIA         int
	NotaOperador   int
	NotaGeral      float64
	AvaliadoEm     time.Time
}

type RelNota struct {
	Nota     int
	IA       int
	Operador int
	Total    int
}

type RelDia struct {
	Dia       string
	Conversas int
	Mensagens int
}

type Limites struct {
	ID                   int       `json:"id"`
	MaxCanais            int       `json:"max_canais"`
	MaxAtendentes        int       `json:"max_atendentes"`
	MaxCampanhasAtivas   int       `json:"max_campanhas_ativas"`
	CampanhaLimiteDiario int       `json:"campanha_limite_diario"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type Integration struct {
	ID                    int       `json:"id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	Method                string    `json:"method"`
	URL                   string    `json:"url"`
	Headers               string    `json:"headers"`
	Parameters            string    `json:"parameters"`
	RequiresIdentity      string    `json:"requires_identity,omitempty"`
	AllowedIdentities     []string  `json:"allowed_identities,omitempty"`
	IdentityConfig        string    `json:"identity_config,omitempty"`
	IdentityVerifyURL     string    `json:"identity_verify_url,omitempty"`
	IdentityVerifyMethod  string    `json:"identity_verify_method,omitempty"`
	IdentityVerifyParam   string    `json:"identity_verify_param,omitempty"`
	IdentityMatchField    string    `json:"identity_match_field,omitempty"`
	IdentityClientIDField string    `json:"identity_client_id_field,omitempty"`
	CreatedAt             time.Time `json:"created_at"`
}

type Skill struct {
	ID                int       `json:"id"`
	TenantID          int       `json:"tenant_id,omitempty"`
	Nome              string    `json:"nome"`
	Descricao         string    `json:"descricao"`
	InstrucaoCompleta string    `json:"instrucao_completa"`
	Categoria         string    `json:"categoria"`
	Ativa             bool      `json:"ativa"`
	AgentIDs          []int     `json:"agent_ids"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type MCPServer struct {
	ID                  int                    `json:"id"`
	TenantID            int                    `json:"tenant_id,omitempty"`
	Nome                string                 `json:"nome"`
	Descricao           string                 `json:"descricao"`
	Transport           string                 `json:"transport"`
	Command             string                 `json:"command"`
	Args                []string               `json:"args"`
	Env                 map[string]string      `json:"env"`
	URL                 string                 `json:"url"`
	Headers             map[string]string      `json:"headers"`
	Ativo               bool                   `json:"ativo"`
	Conectado           bool                   `json:"conectado"`
	UltimoErro          string                 `json:"ultimo_erro"`
	ServerName          string                 `json:"server_name"`
	ServerVersion       string                 `json:"server_version"`
	Capabilities        map[string]interface{} `json:"capabilities"`
	UltimaConexao       *time.Time             `json:"ultima_conexao,omitempty"`
	UltimaSincronizacao *time.Time             `json:"ultima_sincronizacao,omitempty"`
	AgentIDs            []int                  `json:"agent_ids"`
	ToolCount           int                    `json:"tool_count,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

type MCPTool struct {
	ID             int                    `json:"id"`
	TenantID       int                    `json:"tenant_id,omitempty"`
	ServerID       int                    `json:"server_id"`
	ServerNome     string                 `json:"server_nome,omitempty"`
	Transport      string                 `json:"transport,omitempty"`
	Command        string                 `json:"command,omitempty"`
	Args           []string               `json:"args,omitempty"`
	Env            map[string]string      `json:"env,omitempty"`
	URL            string                 `json:"url,omitempty"`
	Headers        map[string]string      `json:"headers,omitempty"`
	Name           string                 `json:"name"`
	DisplayName    string                 `json:"display_name"`
	Description    string                 `json:"description"`
	InputSchema    map[string]interface{} `json:"input_schema"`
	Ativa          bool                   `json:"ativa"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type Plano struct {
	ID          int            `json:"id"`
	TenantOwner *int           `json:"tenant_owner"`
	Nome        string         `json:"nome"`
	Preco       float64        `json:"preco"`
	Intervalo   string         `json:"intervalo"`
	Ativo       bool           `json:"ativo"`
	Recursos    []PlanoRecurso `json:"recursos,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type PlanoRecurso struct {
	ID      int     `json:"id"`
	PlanoID int     `json:"plano_id"`
	Recurso string  `json:"recurso"`
	Limite  float64 `json:"limite"`
}

type EffectiveLimits struct {
	TenantID   int                `json:"tenant_id"`
	PlanoID    *int               `json:"plano_id"`
	PlanoNome  string             `json:"plano_nome"`
	Status     string             `json:"status"`
	TrialUntil *time.Time         `json:"trial_until"`
	Recursos   map[string]float64 `json:"recursos"`
	Uso        map[string]float64 `json:"uso"`
}

type TenantPlanAssignment struct {
	PlanoID        *int               `json:"plano_id"`
	TrialUntil     *time.Time         `json:"trial_until"`
	Status         string             `json:"status"`
	LimitsOverride map[string]float64 `json:"limits_override"`
}

type Agent struct {
	ID               int            `json:"id"`
	TenantID         int            `json:"tenant_id,omitempty"`
	Name             string         `json:"name"`
	SystemPrompt     string         `json:"system_prompt"`
	ProviderID       *int           `json:"provider_id"`
	ProviderName     string         `json:"provider_name,omitempty"`
	TemperatureLevel string         `json:"temperature_level"` // preciso | equilibrado | criativo
	Knowledge        string         `json:"knowledge"`
	KnowledgeLinks   []string       `json:"knowledge_links"`
	KnowledgeContent string         `json:"knowledge_content"`
	SetorIDs         []int          `json:"setor_ids"`
	ToolIDs          []int          `json:"tool_ids"`
	MenuOpcoes       string         `json:"menu_opcoes"`
	Active           bool           `json:"active"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

type Chatflow struct {
	ID        int       `json:"id"`
	TenantID  int       `json:"tenant_id,omitempty"`
	Nome      string    `json:"nome"`
	Version   int       `json:"version"`
	Ativo     bool      `json:"ativo"`
	Config    string    `json:"config"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatflowStep struct {
	ID         int       `json:"id"`
	ChatflowID int       `json:"chatflow_id"`
	Ord        int       `json:"ord"`
	Tipo       string    `json:"tipo"`
	Config     string    `json:"config"`
	CreatedAt  time.Time `json:"created_at"`
}

type AgentChatflowLink struct {
	AgentID    int  `json:"agent_id"`
	ChatflowID *int `json:"chatflow_id"`
}

type ConversationChatflowState struct {
	ChatflowID int               `json:"chatflow_id"`
	StepOrd    *int              `json:"step_ord"`
	Vars       map[string]string `json:"vars"`
}

type Conversation struct {
	ID            int       `json:"id"`
	TenantID      int       `json:"tenant_id,omitempty"`
	ChannelID     *int      `json:"channel_id,omitempty"`
	AgentID       *int      `json:"agent_id"`
	AgentName     string    `json:"agent_name,omitempty"`
	Channel       string    `json:"channel"`
	CustomerName  string    `json:"customer_name"`
	CustomerPhone string    `json:"customer_phone"`
	AvatarURL     string    `json:"avatar_url"`
	Status        string    `json:"status"` // ai | human | em_atendimento | closed
	Typing        bool      `json:"typing"`
	AtendenteID   *int      `json:"atendente_id"`
	AtendenteName string    `json:"atendente_name,omitempty"`
	Origem        string    `json:"origem,omitempty"`
	SetorID       *int      `json:"setor_id"`
	SetorName     string    `json:"setor_name,omitempty"`
	SetorCor      string    `json:"setor_cor,omitempty"`
	Motivo        string    `json:"motivo,omitempty"`
	Satisfacao    string    `json:"satisfacao,omitempty"`
	Hash          string    `json:"hash"`
	BotBlocked    bool      `json:"bot_blocked"`
	AISummary     string    `json:"ai_summary,omitempty"`
	AISummaryAt   *time.Time `json:"ai_summary_at,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Avaliacao struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	NotaIA         *int      `json:"nota_ia"`
	NotaOperador   *int      `json:"nota_operador"`
	Comentario     string    `json:"comentario"`
	CreatedAt      time.Time `json:"created_at"`
}

type LGPDConsent struct {
	ID             int       `json:"id"`
	TenantID       int       `json:"tenant_id"`
	ContatoID      *int      `json:"contato_id"`
	ConversationID *int      `json:"conversation_id"`
	Scope          string    `json:"scope"`
	BaseLegal      string    `json:"base_legal"`
	Aceito         bool      `json:"aceito"`
	Canal          string    `json:"canal"`
	Token          string    `json:"token"`
	IP             string    `json:"ip"`
	Data           time.Time `json:"data"`
}

type LGPDPolitica struct {
	ID             int        `json:"id"`
	TenantID       int        `json:"tenant_id"`
	Version        int        `json:"version"`
	Tipo           string     `json:"tipo"`
	Texto          string     `json:"texto"`
	URL            string     `json:"url"`
	VigenciaInicio time.Time  `json:"vigencia_inicio"`
	VigenciaFim    *time.Time `json:"vigencia_fim"`
	Active         bool       `json:"active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AuditLog struct {
	ID         int                    `json:"id"`
	TenantID   int                    `json:"tenant_id"`
	UsuarioID  *int                   `json:"usuario_id"`
	Usuario    string                 `json:"usuario,omitempty"`
	Acao       string                 `json:"acao"`
	Recurso    string                 `json:"recurso"`
	EntidadeID *int                   `json:"entidade_id"`
	IP         string                 `json:"ip"`
	Data       time.Time              `json:"data"`
	Metadados  map[string]interface{} `json:"metadados,omitempty"`
}

type Message struct {
	ID             int       `json:"id"`
	TenantID       int       `json:"tenant_id,omitempty"`
	ConversationID int       `json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	Source         string    `json:"source"` // ai | human | customer | system
	WAMsgID        string    `json:"wa_msg_id"`
	MediaType      string    `json:"media_type"` // image | audio | video | document
	MediaB64       string    `json:"media_b64"`
	MediaMime      string    `json:"media_mime"`
	MediaName      string    `json:"media_name"`
	Lat            float64   `json:"lat"`
	Lng            float64   `json:"lng"`
	ContactName    string    `json:"contact_name"`
	ContactTel     string    `json:"contact_tel"`
	CreatedAt      time.Time `json:"created_at"`
}

type LogEntry struct {
	ID        int                    `json:"id"`
	TenantID  int                    `json:"tenant_id,omitempty"`
	Ts        time.Time              `json:"ts"`
	Level     string                 `json:"level"`
	Module    string                 `json:"module"`
	Message   string                 `json:"message"`
	Extra     map[string]interface{} `json:"extra"`
	Traceback string                 `json:"traceback"`
	RequestID string                 `json:"request_id"`
}

type Agendamento struct {
	ID              int             `json:"id"`
	TenantID        int             `json:"tenant_id,omitempty"`
	Nome            string          `json:"nome"`
	Descricao       string          `json:"descricao"`
	Tipo            string          `json:"tipo"`
	Quando          string          `json:"quando"`
	Acao            string          `json:"acao"`
	Payload         json.RawMessage `json:"payload"`
	Status          string          `json:"status"`
	MaxExecucoes    *int            `json:"max_execucoes"`
	Execucoes       int             `json:"execucoes"`
	ProximaExecucao *time.Time      `json:"proxima_execucao"`
	UltimaExecucao  *time.Time      `json:"ultima_execucao"`
	Resultado       string          `json:"resultado"`
	Erro            string          `json:"erro"`
	CanalID         *int            `json:"canal_id"`
	ContatoID       *int            `json:"contato_id"`
	Telefone        string          `json:"telefone"`
	AgenteID        *int            `json:"agente_id"`
	CriadoPor       *int            `json:"criado_por"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
