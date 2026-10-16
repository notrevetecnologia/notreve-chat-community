import type { PaginationMeta } from '../../shared/Pagination'

export type Agent = {
  id: number
  name: string
  system_prompt: string
  provider_id?: number | null
  provider_name?: string
  temperature_level: string
  knowledge: string
  knowledge_links: string[]
  knowledge_chars: number
  setor_ids: number[]
  tool_ids: number[]
  menu_opcoes: string
  active: boolean
}

export type Provider = {
  id: number
  name: string
  provider_type: string
  model: string
  active: boolean
  capacidade: string
}

export type Sector = { id: number; name: string }

export type Tool = { id: number; name: string; description: string }

export type Channel = {
  id: number
  name: string
  type: string
  active: boolean
  agent_id?: number | null
}

export type Options = {
  providers: Provider[]
  sectors: Sector[]
  tools: Tool[]
  channels: Channel[]
}

export type Envelope<T> = { data?: T; meta?: PaginationMeta; error?: { message?: string } }
export type AgentResponse = { items: Agent[]; options: Options }

export type Draft = {
  name: string
  system_prompt: string
  provider_id: number | null
  temperature_level: string
  knowledge: string
  knowledge_links: string
  setor_ids: number[]
  tool_ids: number[]
  menu_opcoes: string
  active: boolean
}

export const emptyOptions = (): Options => ({
  providers: [],
  sectors: [],
  tools: [],
  channels: [],
})

export const emptyDraft = (): Draft => ({
  name: "",
  system_prompt: "",
  provider_id: null,
  temperature_level: "equilibrado",
  knowledge: "",
  knowledge_links: "",
  setor_ids: [],
  tool_ids: [],
  menu_opcoes: "",
  active: true,
})

export function channelLabel(type: string): string {
  if (type === "whatsapp") return "WhatsApp"
  if (type === "telegram") return "Telegram"
  if (type === "webchat") return "Webchat"
  return type
}

export function draftFromAgent(agent: Agent): Draft {
  return {
    name: agent.name,
    system_prompt: agent.system_prompt,
    provider_id: agent.provider_id ?? null,
    temperature_level: agent.temperature_level || "equilibrado",
    knowledge: agent.knowledge,
    knowledge_links: agent.knowledge_links.join("\n"),
    setor_ids: agent.setor_ids || [],
    tool_ids: agent.tool_ids || [],
    menu_opcoes: agent.menu_opcoes || "",
    active: agent.active,
  }
}

export function draftToBody(draft: Draft) {
  return {
    name: draft.name.trim(),
    system_prompt: draft.system_prompt,
    provider_id: draft.provider_id,
    temperature_level: draft.temperature_level,
    knowledge: draft.knowledge,
    knowledge_links: draft.knowledge_links
      .split("\n")
      .map((link) => link.trim())
      .filter(Boolean),
    setor_ids: draft.setor_ids,
    tool_ids: draft.tool_ids,
    menu_opcoes: draft.menu_opcoes,
    active: draft.active,
  }
}

export async function request<T>(
  url: string,
  options?: RequestInit,
): Promise<T> {
  const response = await fetch(url, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(options?.headers ?? {}),
    },
  })
  const payload = (await response.json()) as Envelope<T>
  if (!response.ok)
    throw new Error(
      payload.error?.message || "Não foi possível concluir a operação.",
    )
  return payload.data as T
}

export async function requestEnvelope<T>(
  url: string,
  options?: RequestInit,
): Promise<{ data: T; meta: PaginationMeta }> {
  const response = await fetch(url, {
    ...options,
    credentials: "same-origin",
    headers: {
      "Content-Type": "application/json",
      ...(options?.headers ?? {}),
    },
  })
  const payload = (await response.json()) as Envelope<T>
  if (!response.ok)
    throw new Error(
      payload.error?.message || "Não foi possível concluir a operação.",
    )
  return { data: payload.data as T, meta: payload.meta || {} }
}
