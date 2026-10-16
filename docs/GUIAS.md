# Guia — IA, Voz, Imagem e Documentos (como configurar)

Este guia explica como o sistema lida com cada tipo de conteúdo que o cliente
manda (texto, áudio/voz, imagem, PDF/documento) e onde configurar cada coisa.

## Como saber o que cada "IA" faz

Cada provedor de IA (menu **IA**) tem uma **capacidade**:

| Capacidade | O que a IA entende | Exemplos |
|---|---|---|
| **só texto** (padrão) | Apenas texto digitado | DeepSeek Flash, Groq llama, GPT-3.5 |
| **multimodal** | Texto **+ imagem + PDF/documento** (visão) | Gemini Flash, GPT-4o, Claude (com adaptador) |
| **STT (voz)** | **Converte áudio em texto** (transcrição) | Groq Whisper, OpenAI Whisper |

**Regra de ouro:**
- **Visão NÃO ouve áudio** — modelo multimodal enxerga imagem/PDF, mas **não entende voz**.
- **Voz precisa de STT** (transcrição) separado, em qualquer caso.
- Texto funciona com qualquer provedor.

No menu **IA**, cada provedor mostra um selo: 👁️ **multimodal** ou 📝 **só texto**.
No menu **Agentes → Editar → IA**, você escolhe qual provedor o agente usa.

## Áudio / voz do cliente (STT)

Quando o cliente manda áudio/nota de voz no WhatsApp:
1. A API entrega o áudio em base64 no webhook (entrega de mídia habilitada por canal).
2. O sistema chama o **STT** (transcrição) configurado e transforma o áudio em texto.
3. A IA responde **sobre o que foi falado**.

**Onde configurar (Config → Geral → "Transcrição de áudio (STT)"):**
- **Endpoint STT (base URL)**: URL do endpoint OpenAI-compatível de transcrição.
- **Chave STT**: token da API.
- **Modelo STT**: modelo de transcrição.

**Exemplo — Groq Whisper (grátis, rápido):**
- Base: `https://api.groq.com/openai/v1`
- Chave: token da Groq (`gsk_...`) — console.groq.com
- Modelo: `whisper-large-v3`

**Exemplo — OpenAI:**
- Base: `https://api.openai.com/v1`
- Chave: token da OpenAI
- Modelo: `whisper-1`

> Se o STT **não estiver configurado**, a IA avisa com educação que não consegue
> ouvir áudio e pede pro cliente descrever em texto.

> Com um token da **OpenAI**, a mesma chave serve pra chat (visão) **e** STT —
> não precisa de Groq. Groq só cobre STT + chat de texto (não tem visão).

## Imagem e PDF/documento do cliente

Quando o cliente manda **foto, imagem, PDF ou documento**:
1. A API entrega o arquivo em base64 no webhook.
2. Se o provedor do agente for **multimodal**, o arquivo é enviado pro LLM e a IA
   **"vê"** e descreve/analisa.
3. Se o provedor for **só texto**, a IA avisa que não consegue visualizar e pede
   pra descrever.

**Onde configurar (menu IA → + IA):**
- Crie um provedor com **capacidade = multimodal** (ex.: Gemini, GPT-4o).
- No agente (**Agentes → Editar → IA**), selecione esse provedor.

**Exemplos de provedor multimodal:**
- **Gemini (Google)** — tem plano grátis:
  - Base: `https://generativelanguage.googleapis.com/v1beta/openai`
  - Modelo: `gemini-2.0-flash` (ou `gemini-2.5-flash`)
- **GPT-4o (OpenAI)**:
  - Base: `https://api.openai.com/v1`
  - Modelo: `gpt-4o`

> O **vídeo** também é tratado como mídia; se o provedor for multimodal, o
> sistema tenta enviar. Caso não funcione, a IA avisa.

## Resumo do fluxo de mídia

```
Cliente envia → API (base64) → webhook → processador
  ├─ áudio   → STT (Groq/OpenAI Whisper) → texto → IA responde
  ├─ imagem/PDF → se agente multimodal → LLM "vê" → IA responde
  │              senão → IA avisa que não vê (pede descrição)
  └─ texto   → IA responde normal
```

## Notas de manutenção

- A **entrega de mídia base64** é configurada por instância (`/session/media/config`
  → `base64`), feita automaticamente ao conectar um canal.
- A configuração de STT fica em `settings` (`stt_base_url`, `stt_api_key`, `stt_model`).
- Se `stt_base_url` estiver vazio, o sistema tenta usar o provider do agente
  (base + chave) com modelo `whisper-1` — só funciona se o provider tiver STT.
