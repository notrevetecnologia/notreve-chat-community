# Arquitetura de multi-tenancy e white-label

Diagnóstico arquitetural para transformar o Notreve Chat Community de uma instalação
por VPS em uma plataforma capaz de atender vários clientes com isolamento,
planos e identidade visual própria.

Este documento é a decisão de arquitetura e o contrato de segurança do produto.
As migrations de tenant e as proteções de escopo já estão no código; a
ativação completa do control plane, planos e cobrança continua sendo uma etapa
separada para não misturar operação da empresa com infraestrutura da Notreve.

## Decisão recomendada

Adotar inicialmente o modelo **shared application + shared PostgreSQL**, com
`tenant_id` obrigatório nos dados de negócio e isolamento em camadas:

1. contexto de tenant resolvido antes de acessar dados;
2. repositórios SQL que sempre recebem o tenant;
3. chaves estrangeiras e índices que preservam a fronteira entre tenants;
4. Row-Level Security do PostgreSQL como defesa adicional;
5. jobs, cache, sessões, arquivos, logs e webhooks com namespace do tenant.

A instalação atual continua sendo o **tenant 1**. Os dados existentes são
associados ao tenant 1, preservando IDs, conversas, canais, agentes, usuários e
histórico. Não é necessário criar uma segunda instalação nem duplicar o banco
para iniciar a migração.

Durante a transição, o modo atual de uma VPS por cliente pode continuar
existindo. Nesse caso, cada banco isolado também representa um tenant 1 local.
O modelo compartilhado só deve ser ativado depois que os filtros, o contexto e
os testes de isolamento estiverem prontos.

## Diagnóstico do estado atual

O núcleo de tenant já foi criado e a instalação atual é o tenant 1. O que ainda
está em evolução é a camada de control plane e a eliminação dos últimos
fallbacks legados. A base inicial era operacional, por instalação:

- `settings` é singleton com `id=1`;
- `limites` é singleton com `id=1`;
- `modulos` é uma lista global, carregada no boot e mantida em cache no `Server`;
- a sessão guarda apenas o ID do usuário no cookie/Redis;
- o login procura `usuarios.username` globalmente;
- vários métodos de `internal/store` fazem `List`, `Count`, `Get` e `Update` sem
  receber tenant;
- canais públicos são encontrados por `public_id`, `token` ou `instance_id`,
  sem um registro de domínio/tenant explícito;
- jobs de distribuição, satisfação, follow-up, paciência e campanha consultam
  o conjunto global de dados;
- cache Redis, mapas em memória, arquivos de mídia e logs ainda não têm
  namespace de tenant;
- várias restrições `UNIQUE` impediriam dois clientes de usarem o mesmo nome,
  telefone, usuário ou chave de mensagem.

O risco principal não é apenas uma query esquecida. É permitir que um ID válido
de um tenant seja usado em uma rota do outro tenant. Por isso o tenant deve
participar do contexto, das queries, das relações e dos testes.

## Modelo de domínio

### Plataforma

Dados controlados pela Notreve, não pertencentes a um cliente específico:

- catálogo de planos e recursos;
- catálogo de módulos;
- contas e permissões de plataforma;
- domínios globais e configuração do provisionamento;
- auditoria de ações de super_admin;
- conectores ou credenciais compartilhadas explicitamente marcados como
  plataforma;
- estado de cobrança e provisionamento.

### Tenant

Um tenant é o workspace isolado de uma empresa cliente. Deve possuir pelo menos:

- `id` estável;
- `slug` único;
- nome comercial e status (`trial`, `active`, `suspended`, `closed`);
- plano e eventuais limites customizados;
- fuso horário e locale;
- configuração de retenção e LGPD;
- datas de criação, suspensão e encerramento.

O tenant é o dono dos canais, contatos, agentes, bases de conhecimento,
conversas, mensagens, equipe, integrações, campanhas, relatórios e branding.

### Identidade e vínculo

O desenho de longo prazo deve separar:

- uma identidade de login, que pode pertencer a mais de um tenant;
- uma associação identidade-tenant, com papel, permissões e status;
- acesso de plataforma, para quem trabalha na operação da Notreve.

Como migração de menor risco, `usuarios` pode receber `tenant_id` primeiro,
mantendo usuário e associação na mesma tabela. A separação em `identidades` e
`tenant_memberships` pode vir depois, sem bloquear o isolamento inicial.

`super_admin` deve ser tratado como papel de plataforma. Ele pode consultar ou
operar tenants somente por uma ação explícita de troca de contexto, registrada
em auditoria. Não deve receber acesso cruzado apenas porque uma rota esqueceu
um filtro.

## Mapeamento das tabelas atuais

No modelo alvo, IDs numéricos podem continuar globalmente únicos para reduzir
risco de migração. `tenant_id` continua obrigatório para autorização, filtro,
índices, auditoria e validação de relações.

| Tabela atual | `tenant_id` | Tratamento recomendado |
|---|---:|---|
| `settings` | Sim | Uma configuração por tenant. O singleton `id=1` vira a configuração do tenant 1. Separar depois campos de infraestrutura da plataforma. |
| `providers` | Sim | Provedores/modelos disponíveis ao tenant; API keys devem ser secretas e criptografadas. Catálogo compartilhado deve ser outra entidade. |
| `agents` | Sim | Agentes, prompts, conhecimento, tools e vínculos pertencem ao tenant. Validar `provider_id`, setores e tools no mesmo tenant. |
| `channels` | Sim | Cada WhatsApp/webchat pertence a um tenant. `public_id`, `instance_id` e identificadores usados antes da autenticação precisam ser globalmente não ambíguos. |
| `integrations` | Sim | Integrações e credenciais são do tenant; nunca reutilizar `name` global como fronteira de segurança. |
| `setores` | Sim | Nomes passam a ser únicos por tenant. |
| `atendentes` | Sim | Equipe operacional do tenant. |
| `atendente_setores` | Sim | Tabela de relação; usar `tenant_id` e validar que os dois lados pertencem ao mesmo tenant. |
| `usuarios` | Sim | Login e permissões do tenant. `username` deve ser único por tenant, ou o login deve exigir domínio/tenant. Acesso de plataforma deve ser separado ou explicitamente marcado. |
| `conversations` | Sim | Fronteira primária de atendimento. Todo acesso por ID precisa conferir tenant. |
| `messages` | Sim | Recomendável manter `tenant_id` para RLS, auditoria e consultas rápidas, com FK composta que impeça divergência em relação à conversa. |
| `avaliacoes` | Sim | Nota pertence à conversa e ao tenant; relatórios não podem agregar dados globais por acidente. |
| `mensagens` | Sim | Templates editáveis do tenant. O catálogo padrão da Notreve deve ser separado ou copiado no provisionamento. `chave` deixa de ser única global. |
| `respostas_rapidas` | Sim | Respostas do tenant; manter também `user_id` para autoria. |
| `contatos` | Sim | Telefone, e-mail e histórico pertencem ao tenant. `phone` deve ser único por tenant, não no banco inteiro. |
| `aprendizados` | Sim | Perguntas e lacunas de conhecimento são dados do tenant. |
| `segmentos` | Sim | Segmentos do tenant; `name` único por tenant. |
| `contato_segmentos` | Sim | Relação com validação de tenant nos dois registros. |
| `grupo_canais` | Sim | Grupo de canais de campanha do tenant. |
| `grupo_canal` | Sim | Relação com validação de tenant entre grupo e canal. |
| `campanhas` | Sim | Campanha, autor, segmento, grupo e canal precisam pertencer ao mesmo tenant. |
| `campanha_envios` | Sim | Envio congelado da campanha; validar campanha, contato e canal no mesmo tenant. |
| `webchat_sessions` | Sim | Sessão pública deve ser resolvida pelo canal/tenant e nunca compartilhar visitante entre clientes. |
| `modulos` | Não diretamente | Transformar em catálogo de módulos da plataforma + tabela de habilitação/override por tenant. O estado atual global não pode continuar governando todos os clientes. |
| `limites` | Não diretamente | Transformar em catálogo de planos + assinatura/limites efetivos por tenant. O registro `id=1` vira os limites do tenant 1. |
| `schema_migrations` | Não | É controle técnico da instalação/banco, não dado de cliente. |

As três tabelas de relação (`atendente_setores`, `contato_segmentos` e
`grupo_canal`) poderiam derivar o tenant por JOIN, mas isso é insuficiente como
única defesa. O desenho recomendado é guardar `tenant_id` e usar validação por
FK composta ou trigger para evitar relações cruzadas.

`messages` e `avaliacoes` também poderiam derivar o tenant da conversa. A cópia
do tenant é recomendada porque são tabelas de alto volume e alto risco de
exposição em relatórios, mídia e exportações. O banco deve impedir que o valor
copiado diverja do pai.

### Unicidades que precisam mudar

As seguintes unicidades hoje são globais e precisam ser revistas:

- `providers.name` → `(tenant_id, name)`;
- `integrations.name` → `(tenant_id, name)`;
- `setores.name` → `(tenant_id, name)`;
- `usuarios.username` → `(tenant_id, username)` quando o login usar domínio;
- `mensagens.chave` → `(tenant_id, chave)`;
- `contatos.phone` → `(tenant_id, phone)`;
- `segmentos.name` → `(tenant_id, name)`.

Já `tenant.slug`, domínio, `channels.public_id` e o identificador usado para
rotear webhook devem ser globalmente únicos, pois são resolvidos antes de uma
sessão autenticada. `instance_id` deve ser globalmente único dentro do
namespace do provedor ou ter a composição explicitamente definida.

## Contexto de tenant

### Resolução da requisição

A ordem recomendada é:

1. validar o host/domínio conhecido no proxy e resolver `tenant_domains`;
2. em rotas públicas, resolver o tenant pelo `public_id`, chave pública ou
   identificador de webhook do canal;
3. em rotas autenticadas, conferir se a sessão possui associação com o tenant
   resolvido;
4. se houver super_admin, exigir tenant escolhido explicitamente para entrar na
   área de operação do cliente;
5. se não houver contexto inequívoco, responder `401`/`403` e não tentar usar
   tenant padrão.

Não confiar diretamente em `Host` ou `X-Forwarded-Host` sem uma cadeia de proxy
conhecida. O host deve ser normalizado, comparado a uma lista cadastrada e
protegido contra host header injection.

### Login

O login atual por `username` global não é suficiente. As opções compatíveis são:

- login em domínio do tenant, permitindo `username` único dentro dele;
- login central com `tenant slug + username`;
- identidade global com seleção de workspace após autenticar.

A sessão deve carregar, no mínimo, `user_id`, `tenant_id` ativo, papel,
versão de autorização, momento de emissão e expiração. O valor no cookie deve
continuar opaco. Redis e caches devem usar uma chave que inclua tenant e
versão de sessão, sem aceitar um tenant fornecido pelo navegador como autoridade.

### Webhooks e canais públicos

Webhooks não têm sessão de usuário. A resolução deve ocorrer por credencial
rotacionável ou identificador de canal, e o canal resolvido deve carregar o
tenant. Não usar “primeiro canal ativo” ou fallback global.

Para webchat, `public_id`/chave do widget deve apontar para um único canal. A
sessão do visitante, token, mensagens e encerramento devem validar o canal e o
tenant em cada chamada.

## Isolamento obrigatório

O isolamento deve ser tratado como requisito de segurança, não como convenção.

### Camada HTTP

- middleware resolve e fixa `TenantContext` antes do handler;
- rotas de tenant recusam ausência de contexto;
- IDs vindos de URL são consultados dentro do tenant, nunca globalmente;
- tenant enviado em body/query/header é apenas uma intenção e nunca substitui
  o contexto autenticado;
- respostas, downloads de mídia e exportações repetem a checagem de posse;
- super_admin usa rotas de plataforma ou troca de contexto auditada.

### Camada Store/SQL

- toda função de leitura, contagem, criação, atualização e exclusão recebe
  `tenantID` ou um contexto equivalente;
- remover APIs ambíguas como `ListChannels(ctx)` no caminho compartilhado;
- `UPDATE` e `DELETE` precisam incluir tenant no predicado;
- joins de entidades do tenant precisam validar o tenant de cada lado;
- IDs de outro tenant devem parecer “não encontrado”, não revelar existência;
- índices começam por `tenant_id` nas tabelas de consulta frequente;
- relações importantes usam FKs compostas ou validação transacional.

### Row-Level Security

Depois que todas as queries estiverem escopadas, habilitar RLS como segunda
barreira. A transação deve definir `app.tenant_id` com `SET LOCAL`, evitando
vazamento de estado entre conexões do pool. A conexão de aplicação não deve
ter um bypass permanente. O acesso de plataforma deve ser uma política distinta,
com auditoria e sem virar o comportamento normal do sistema.

### Jobs, cache e arquivos

- jobs devem listar tenants ativos e executar cada ciclo com contexto próprio;
- configurações e limites devem ser carregados por tenant;
- chaves Redis devem usar `tenant:{id}:...`;
- mapas em memória precisam usar tenant + recurso como chave;
- uploads, mídia e exportações devem ficar em caminhos com tenant;
- logs, métricas, tracing e eventos devem conter `tenant_id`, sem incluir segredos;
- rate limits e cotas devem ser contabilizados por tenant e, quando necessário,
  também por canal.

### Testes de isolamento

Antes de habilitar o segundo tenant, criar testes que:

- criem os mesmos nomes, telefones e chaves em tenants diferentes;
- tentem ler, editar, excluir e enviar mensagem usando o ID do outro tenant;
- executem jobs com dados misturados;
- validem webhooks, webchat, mídia, relatórios e exportações;
- reiniciem o app e confirmem que sessão/cache não trocam contexto;
- executem queries com RLS e conexão reutilizada pelo pool.

## Super_admin, admin e permissões

O `admin` é o cliente dono do workspace contratado. Ele pode operar somente o
próprio tenant: equipe, canais, agentes, configurações do atendimento,
integrações e módulos que o plano permitir. Ele não pode criar/editar
`super_admin`, planos, limites de plataforma, SMTP global, provedores WhatsApp
globais, créditos Notreve ou outros tenants. `supervisor` e `atendente`
permanecem limitados ao tenant e às permissões granulares concedidas.

O `super_admin` da Notreve opera **somente o control plane**: empresas,
domínios, planos, módulos, limites, branding/white-label, SMTP, catálogo de
provedores globais, créditos Notreve, provisionamento, auditoria e saúde da
plataforma. Ele não recebe inbox, contatos, conversas, transcrições, bases de
conhecimento ou relatórios de uma empresa como comportamento padrão.

O workspace de cada cliente é uma superfície separada e pertence aos perfis
do tenant. Se, no futuro, houver suporte excepcional da Notreve dentro de um
tenant, ele deve ser um acesso temporário, explicitamente solicitado,
identificado no cabeçalho, limitado ao necessário e auditado; não é uma função
normal do super_admin.

Não esconder a diferença apenas por menu. Cada endpoint deve declarar se é de
plataforma ou de tenant, e uma permissão de plataforma não deve ser inferida de
uma permissão operacional comum.

Na implementação atual, `plataforma_gerenciar` é exclusiva do
`super_admin`; a tela e as rotas de limites/control plane exigem essa permissão.
O `admin` continua com `config_gerenciar` apenas para as configurações do seu
workspace. A UI também não exibe a área de plataforma para o admin.

## Planos, módulos e limites

O modelo atual de `modulos` e `limites` é global. O modelo alvo deve separar:

- `plans`: catálogo de planos da Notreve, com código, nome, status, recursos e
  limites padrão;
- `plan_features`: recursos booleanos ou com nível;
- `plan_limits`: canais, usuários, campanhas ativas, envios/dia, contatos,
  armazenamento, mensagens, tokens de IA, tools, webhooks e retenção;
- `tenant_subscriptions`: plano efetivo, status, trial, início, renovação,
  suspensão e referência externa de cobrança;
- `tenant_feature_overrides` e `tenant_limit_overrides`: exceções auditadas;
- `tenant_modules`: habilitação efetiva por tenant, resultante do plano e de
  overrides do super_admin.

### Catálogo funcional do produto

O catálogo deve ser entendido como capacidades independentes. O super_admin
libera ou suspende cada módulo por empresa; o admin configura apenas o que foi
liberado e os perfis abaixo dele enxergam somente suas permissões granulares.
Usuários/equipe, canais e conversas são a base nativa do produto: o
provisionamento não deve entregar um workspace utilizável sem essas três
capacidades. Os demais módulos são contratáveis e independentes.

| Módulo | O que entrega | Dependências nativas |
|---|---|---|
| `conversas` | Inbox, histórico, respostas, mídia, transferência e encerramento | Usuários, permissões e pelo menos um canal |
| `canais` | WhatsApp, webchat e outros pontos de entrada | Tenant ativo; o canal define a origem do atendimento |
| `equipe` | Usuários, atendentes, setores e filas | Usuário admin inicial |
| `ia` | Provedores, agentes, treinamento e base de conhecimento | Pelo menos um canal para atender |
| `tools` | Ações externas seguras para os agentes, como APIs e automações externas | `ia` + agente com a tool explicitamente vinculada |
| `api` | API pública para abrir conversa, enviar mensagem e webhooks de eventos | `conversas` + chave e escopos da empresa |
| `campanhas` | Importação de contatos, segmentos, grupos de canais, disparos e agenda | `contatos` + `canais` |
| `contatos` | Cadastro, importação, segmentos e histórico de relacionamento | Tenant ativo |
| `relatorios` | Operação, satisfação por IA/atendente/setor e campanhas | Dados dos módulos usados |
| `mensagens` | Respostas rápidas, templates e mensagens automáticas | `conversas` |

`canais`, `equipe` e `conversas` formam a base operacional: sem identidade de
usuário, canal ou caixa de atendimento não existe operação utilizável. Eles são
habilitados no provisionamento e não aparecem como uma opção ambígua para o
admin desligar. `tools` e `api` são módulos distintos mesmo quando aparecem
próximos no menu; uma empresa pode usar API sem liberar tools, ou tools sem
publicar API.

### Experiência por perfil

- `super_admin`: somente control plane — empresas, planos, módulos, limites,
  branding, créditos, SMTP, provedores globais, saúde e auditoria. Não vê nem
  opera conversas, contatos ou dados de clientes como parte do painel normal.
- `admin`: workspace da própria empresa — setup guiado, canais, agentes,
  conhecimento, tools, API, equipe e relatórios que o plano liberar. Não vê
  URLs internas, modelos globais, SMTP, créditos ou outras empresas.
- `supervisor`: operação de equipe, filas, setores, relatórios e conversas
  conforme escopo granular concedido pelo admin.
- `atendente`: atendimento diário, contatos e recursos de conversa autorizados;
  não configura plataforma, módulos, credenciais ou agentes sem permissão.

O setup guiado deve pedir ao admin somente o necessário na ordem: conectar
Notreve IA com um token, criar canal, criar agente, opcionalmente ensinar a
base, organizar equipe e então habilitar tools/API/campanhas conforme o plano.
Configurações internas como endpoint do gateway, modelo padrão, STT, allowlist
global e URL base de webhook pertencem ao control plane, não ao formulário do
admin.

Créditos de IA seguem o mesmo princípio: saldo, consumo, recargas e estornos
devem pertencer ao tenant e ser alteráveis pelo `super_admin`. O admin poderá
escolher entre usar os provedores Notreve (com créditos do plano/saldo) ou
cadastrar uma IA própria, com custos e credenciais sob responsabilidade do
próprio tenant. A tela de créditos, catálogo de provedores globais e SMTP do
control plane ficam na fila da fase de planos; não devem ser colocados na tela
de configurações do admin.

O tenant admin deve conseguir ver consumo e limites, mas não aumentar o próprio
plano. A aplicação precisa distinguir “recurso não contratado”, “limite
atingido” e “erro técnico”. Campanhas, mídia, ferramentas externas e tokens de
IA devem ter contadores por tenant para evitar que um cliente consuma quota de
outro.

Na migração, o registro atual de `limites` deve virar a configuração efetiva do
tenant 1. Os valores não devem ser reinterpretados nem zerados.

## White-label e branding

White-label depende do contexto de tenant. Primeiro resolve-se o tenant;
depois o frontend e o backend carregam a identidade correspondente.

### Branding mínimo

Criar uma configuração de branding por tenant com:

- nome do produto e nome comercial;
- logo principal, logo para fundo escuro e fallback textual;
- favicon e ícones de instalação;
- cor primária, secundária, destaque, fundo, superfície e texto;
- estado de carregamento: logo, texto, animação e cor de fundo;
- título de login, texto de boas-vindas e contato de suporte;
- nível `notreve`, `co_brand` ou `full_white_label`;
- domínio principal e domínios alternativos aprovados.

O React deve receber esses valores no bootstrap e aplicar tokens CSS. As telas
legadas devem receber os mesmos tokens para evitar que o painel antigo fique
com outra marca durante a transição. O webchat pode herdar o branding do tenant
e permitir overrides por canal.

### Segurança do branding

- não aceitar HTML, JavaScript ou CSS arbitrário de clientes;
- validar tipo, tamanho e dimensões de imagens;
- armazenar assets em caminho/bucket com tenant;
- servir assets com MIME e cache corretos;
- manter CSP e impedir carregamento de scripts de terceiros não autorizados;
- validar contraste mínimo e preservar estados de erro/acessibilidade;
- usar branding Notreve como fallback quando configuração estiver incompleta;
- limpar cache por tenant ao publicar alteração.

O carregamento, logo e favicon devem ser configuráveis pelo super_admin, mas não
devem alterar a identidade do control plane usado pela Notreve.

## Migração por etapas

As migrations existentes são imutáveis. Cada etapa abaixo deve gerar novas
migrations numeradas e ser compatível com o estado anterior durante o rollout.

### Etapa 0 — inventário e contrato

- congelar a lista de tabelas, relações, unicidades e fluxos públicos;
- definir `tenant 1`, slug, status e política de domínio;
- definir quais dados são da plataforma e quais são do tenant;
- definir o contrato de `TenantContext`, sessão, bootstrap React e auditoria;
- criar matriz de testes de isolamento antes de liberar o segundo tenant.

### Etapa 1 — tenant 1 sem mudança funcional

- criar `tenants` e registrar a instalação atual como tenant 1;
- adicionar `tenant_id` inicialmente nullable às tabelas de negócio;
- fazer backfill de todos os registros existentes para tenant 1;
- validar órfãos, duplicidades e relações cruzadas;
- criar índices de tenant e adaptar unicidades sem apagar histórico;
- tornar `tenant_id` NOT NULL somente após a validação;
- converter `settings` e `limites` singleton para a representação do tenant 1.

Nenhuma conversa, mensagem, canal, contato ou usuário existente deve ser
recriado com novos IDs.

### Etapa 2 — contexto e repositório

- implementar resolução por domínio, sessão e chave pública;
- colocar tenant na sessão e nas chaves de cache;
- alterar Store e handlers para exigir contexto;
- bloquear fallback global e lookup por ID sem tenant;
- separar rotas de plataforma das rotas do workspace;
- adaptar jobs para iterar tenants ativos com contexto próprio.

### Etapa 3 — isolamento de banco e operação

- adicionar FKs compostas/validações das relações;
- habilitar RLS gradualmente em staging e depois em produção;
- testar conexões reutilizadas, Redis, uploads, mídia e relatórios;
- adicionar auditoria de troca de tenant e operações de super_admin;
- instrumentar métricas por tenant e alertas de erro de contexto.

### Etapa 4 — domínios, login e acesso

- cadastrar domínio padrão e domínios customizados;
- escolher login por domínio ou seleção de workspace;
- migrar `usuarios` para memberships quando necessário;
- manter sessão antiga somente durante a janela de compatibilidade;
- invalidar sessões e tokens incompatíveis após o corte.

### Etapa 5 — planos e limites

- criar catálogo de planos e recursos;
- migrar o limite atual para a assinatura do tenant 1;
- calcular consumo por tenant;
- mover módulos globais para catálogo + habilitação por tenant;
- testar excesso, suspensão, trial e override de super_admin.

### Etapa 6 — white-label

- criar branding e assets por tenant;
- expor branding no bootstrap React e nos templates legados;
- aplicar tokens no shell, login, carregamentos, favicon e webchat;
- publicar primeiro em `co_brand`, depois liberar `full_white_label`;
- validar CSP, cache, contraste e fallback.

### Etapa 7 — migração do painel

Com contexto e branding disponíveis, migrar as áreas restantes para React na
ordem: layout/design system, login, canais, equipe, agentes/treinamento,
integrações/tools, contatos/campanhas, relatórios, configurações e control
plane. Cada tela nova deve nascer com tenant explícito, estados de loading/erro
e permissões do tenant.

### Etapa 8 — segundo tenant e retirada da compatibilidade

- provisionar um tenant de teste com dados sintéticos;
- executar testes cruzados, carga, webhooks e jobs;
- liberar um cliente piloto;
- observar métricas e logs sem habilitar migração em massa automática;
- remover queries globais, singleton de runtime e fallback de tenant somente
  após a janela de segurança e backup verificável.

## Riscos e mitigação

| Risco | Impacto | Mitigação |
|---|---|---|
| Query sem filtro de tenant | Vazamento de conversas, contatos ou credenciais | Contexto obrigatório no Store, revisão SQL, RLS e testes negativos. |
| ID de outro tenant em rota | Alteração ou envio para cliente errado | Predicados com tenant, FKs compostas e resposta indistinta de não encontrado. |
| Lookup público ambíguo | Webhook/webchat roteado para tenant errado | Domínio/chave pública globalmente única e sem fallback. |
| Unicidade global atual | Falha ao criar dados legítimos ou colisão lógica | Índices compostos por tenant, mantendo globalidade apenas nas chaves públicas. |
| `settings`, `limites` e `modulos` globais | Configuração de um cliente afeta todos | Separar configuração da plataforma da configuração efetiva por tenant. |
| Job sem contexto | Mensagem, campanha ou relatório cruzado | Iterar tenants e executar cada ciclo em transação/contexto próprio. |
| Redis/cache sem namespace | Sessão, contador ou branding trocado | Prefixo obrigatório com tenant e invalidação por tenant. |
| RLS mal aplicado no pool | Bloqueio indevido ou vazamento entre requisições | `SET LOCAL` por transação, sem estado de sessão persistente e testes de conexão reutilizada. |
| Super_admin com bypass amplo | Acidente ou abuso com alcance total | Control plane separado, troca explícita, auditoria e confirmação para ações destrutivas. |
| Credenciais em integrações/providers | Vazamento entre clientes | Escopo por tenant, criptografia, redaction em logs e rotação. |
| Migração de dados incompleta | Registros órfãos ou histórico perdido | Backfill idempotente, contagens antes/depois, constraints após validação e backup. |
| Dois frontends durante a transição | Branding e permissões divergentes | Contrato JSON único, tokens compartilhados e migração por domínio funcional. |
| White-label com CSS/script arbitrário | XSS, phishing e quebra de acessibilidade | Tokens controlados, assets validados e CSP. |
| Mistura de VPS isolada e banco compartilhado | Operação difícil de explicar | Definir deployment mode por instalação e não habilitar compartilhamento sem observabilidade. |

## Critério de pronto para o segundo tenant

O segundo tenant só deve ser ativado quando todos os itens abaixo forem
verdadeiros:

- nenhuma tabela de negócio fica sem escopo definido;
- nenhuma função crítica acessa dados sem `TenantContext`;
- login, webhooks e webchat resolvem tenant sem fallback;
- jobs, Redis, mídia, logs e métricas são tenant-aware;
- dados existentes foram migrados para tenant 1 e reconciliados;
- RLS ou defesa equivalente foi validada;
- super_admin tem superfície de plataforma e auditoria;
- planos, módulos e limites não são globais;
- branding funciona no React, legado, login e webchat;
- testes de tentativa de acesso cruzado passam em CI e staging;
- backup e rollback da migração foram exercitados.
