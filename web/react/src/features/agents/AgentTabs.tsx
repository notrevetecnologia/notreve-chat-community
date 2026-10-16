import { useCallback, useEffect, useState } from "react";
import {
  channelLabel,
  request,
  type Draft,
  type Options,
} from "./agentShared";

type ChangeDraft = <K extends keyof Draft>(key: K, value: Draft[K]) => void;

type TabProps = {
  draft: Draft;
  onChange: ChangeDraft;
  options: Options;
};

function toggleNumber(list: number[], id: number): number[] {
  return list.includes(id)
    ? list.filter((item) => item !== id)
    : [...list, id];
}

function Help({ children }: { children: string }) {
  return <small className="agent-help">{children}</small>;
}

function EmptyNote({ children }: { children: string }) {
  return <p className="agent-tab-empty">{children}</p>;
}

export function AgentBehaviorTab({
  draft,
  onChange,
  options,
  onOpenProviders,
}: TabProps & {
  onOpenProviders?: () => void;
}) {
  return (
    <div className="agent-field-grid">
      <label className="agent-field">
        <span>Nome do agente *</span>
        <input
          autoFocus
          value={draft.name}
          onChange={(event) => onChange("name", event.target.value)}
          placeholder="Ex.: Concierge financeiro"
        />
        <Help>Nome exibido para a equipe. Obrigatório.</Help>
      </label>
      <label className="agent-field">
        <span>Provedor de IA</span>
        <select
          value={draft.provider_id ?? ""}
          onChange={(event) =>
            onChange(
              "provider_id",
              event.target.value ? Number(event.target.value) : null,
            )
          }
        >
          <option value="">Provedor padrão</option>
          {options.providers.map((provider) => (
            <option key={provider.id} value={provider.id}>
              {provider.name}
              {provider.active ? "" : " · pausado"}
            </option>
          ))}
        </select>
        <Help>Modelo que gera as respostas. Sem provedor a IA não responde.</Help>
      </label>
      <label className="agent-field">
        <span>Estilo de resposta</span>
        <select
          value={draft.temperature_level}
          onChange={(event) =>
            onChange("temperature_level", event.target.value)
          }
        >
          <option value="preciso">Preciso</option>
          <option value="equilibrado">Equilibrado</option>
          <option value="criativo">Criativo</option>
        </select>
        <Help>Preciso = direto e técnico; Criativo = mais livre e caloroso.</Help>
      </label>
      <label className="agent-field agent-field-check">
        <input
          type="checkbox"
          checked={draft.active}
          onChange={(event) => onChange("active", event.target.checked)}
        />
        <span>
          Agente ativo
          <Help>Pausado deixa de responder sem apagar a configuração.</Help>
        </span>
      </label>
      <label className="agent-field agent-field-wide">
        <span>Instrução principal</span>
        <textarea
          rows={8}
          value={draft.system_prompt}
          onChange={(event) => onChange("system_prompt", event.target.value)}
          placeholder="Ex.: Você é o assistente da Loja X. Fale de forma simples, nunca invente preços e sempre confirme o pedido antes de finalizar."
        />
        <Help>
          Personalidade, tom e limites do agente. É o texto que a IA lê antes de
          cada resposta — quanto mais claro, melhor.
        </Help>
      </label>
      {onOpenProviders && (
        <div className="agent-field-wide entity-cross-links">
          <span>Precisa conectar ou trocar o modelo de IA?</span>
          <button type="button" onClick={onOpenProviders}>
            Gerenciar provedores de IA →
          </button>
        </div>
      )}
    </div>
  );
}

export function AgentToolsTab({ draft, onChange, options }: TabProps) {
  return (
    <div className="agent-tab-body">
      <p className="agent-tab-lead">
        Ferramentas são ações que a IA pode executar (consultar um sistema,
        enviar um dado). Marque só as que este agente deve usar.
      </p>
      {options.tools.length ? (
        <div className="agent-choice-list">
          {options.tools.map((tool) => (
            <label key={tool.id} className="agent-choice">
              <input
                type="checkbox"
                checked={draft.tool_ids.includes(tool.id)}
                onChange={(event) =>
                  onChange(
                    "tool_ids",
                    event.target.checked
                      ? toggleNumber(draft.tool_ids, tool.id)
                      : draft.tool_ids.filter((item) => item !== tool.id),
                  )
                }
              />
              <span>
                <b>{tool.name}</b>
                <small>{tool.description || "Sem descrição."}</small>
              </span>
            </label>
          ))}
        </div>
      ) : (
        <EmptyNote>
          Nenhuma ferramenta cadastrada. Crie integrações em Integrações.
        </EmptyNote>
      )}
    </div>
  );
}

type Skill = {
  id: number;
  nome: string;
  descricao: string;
  instrucao_completa: string;
  categoria: string;
  ativa: boolean;
  agent_ids: number[];
};

export function AgentSkillsTab({ agentId, onOpenSkills }: { agentId: number | null; onOpenSkills?: () => void }) {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyID, setBusyID] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await request<{ items: Skill[] }>("/api/react/skills?limit=100");
      setSkills(payload.items || []);
      setError(null);
    } catch (loadError) {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Não foi possível carregar as skills.",
      );
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const toggle = async (skill: Skill) => {
    if (!agentId) return;
    const next = skill.agent_ids.includes(agentId)
      ? skill.agent_ids.filter((item) => item !== agentId)
      : [...skill.agent_ids, agentId];
    setBusyID(skill.id);
    try {
      await request(`/api/react/skills/${skill.id}`, {
        method: "PATCH",
        body: JSON.stringify({
          nome: skill.nome,
          descricao: skill.descricao,
          instrucao_completa: skill.instrucao_completa,
          categoria: skill.categoria,
          ativa: skill.ativa,
          agent_ids: next,
        }),
      });
      setSkills((current) =>
        current.map((item) =>
          item.id === skill.id ? { ...item, agent_ids: next } : item,
        ),
      );
      setError(null);
    } catch (toggleError) {
      setError(
        toggleError instanceof Error
          ? toggleError.message
          : "Não foi possível atualizar a skill.",
      );
    } finally {
      setBusyID(null);
    }
  };

  return (
    <div className="agent-tab-body">
      <p className="agent-tab-lead">
        Skills são pacotes de instruções que a IA carrega sob demanda. Marque as
        que este agente pode invocar.
      </p>
      {onOpenSkills && (
        <div className="entity-cross-links">
          <span>Quer criar ou editar instruções reutilizáveis?</span>
          <button type="button" onClick={onOpenSkills}>
            Gerenciar skills →
          </button>
        </div>
      )}
      {!agentId && (
        <EmptyNote>Salve o agente antes de vincular skills.</EmptyNote>
      )}
      {error && <div className="agent-inline-error">{error}</div>}
      {loading ? (
        <p className="agent-tab-empty">Carregando skills…</p>
      ) : skills.length ? (
        <div className="agent-choice-list">
          {skills.map((skill) => (
            <label key={skill.id} className="agent-choice">
              <input
                type="checkbox"
                disabled={!agentId || busyID === skill.id}
                checked={!!agentId && skill.agent_ids.includes(agentId)}
                onChange={() => void toggle(skill)}
              />
              <span>
                <b>{skill.nome}</b>
                <small>{skill.descricao || skill.categoria}</small>
              </span>
            </label>
          ))}
        </div>
      ) : (
        <EmptyNote>
          Nenhuma skill cadastrada. Crie skills na seção Skills.
        </EmptyNote>
      )}
    </div>
  );
}

type KnowledgeSource = {
  id: number;
  kind: string;
  name: string;
  source_url?: string;
  content_chars: number;
  status: string;
};

type KnowledgeView = {
  agent_id: number;
  sources: KnowledgeSource[];
  usage: { sources: number; chars: number };
  limits: { max_sources_per_agent: number; max_total_chars: number };
};

export function AgentKnowledgeTab({
  draft,
  onChange,
  agentId,
  onOpenKnowledge,
}: {
  draft: Draft;
  onChange: ChangeDraft;
  agentId: number | null;
  onOpenKnowledge?: (id: number) => void;
}) {
  const [data, setData] = useState<KnowledgeView | null>(null);
  const [kind, setKind] = useState<"text" | "site">("text");
  const [text, setText] = useState("");
  const [url, setUrl] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!agentId) {
      setData(null);
      return;
    }
    try {
      const payload = await request<KnowledgeView>(
        `/api/react/knowledge?agent_id=${agentId}`,
      );
      setData({
        ...payload,
        sources: payload.sources || [],
        usage: payload.usage || { sources: 0, chars: 0 },
        limits: payload.limits || { max_sources_per_agent: 0, max_total_chars: 0 },
      });
      setError(null);
    } catch (loadError) {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Não foi possível carregar a base de conhecimento.",
      );
    }
  }, [agentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const add = async () => {
    if (!agentId) return;
    if (kind === "text" && !text.trim()) {
      setError("Cole o texto que será usado no treinamento.");
      return;
    }
    if (kind === "site" && !url.trim()) {
      setError("Informe a URL do site que será usado como fonte.");
      return;
    }
    setBusy(true);
    try {
      await request("/api/react/knowledge", {
        method: "POST",
        body: JSON.stringify({
          agent_id: agentId,
          kind,
          name: "",
          source_url: kind === "site" ? url.trim() : "",
          text: kind === "text" ? text.trim() : "",
        }),
      });
      setText("");
      setUrl("");
      setError(null);
      await load();
    } catch (addError) {
      setError(
        addError instanceof Error
          ? addError.message
          : "Não foi possível processar a fonte.",
      );
    } finally {
      setBusy(false);
    }
  };

  const remove = async (source: KnowledgeSource) => {
    if (!window.confirm(`Remover “${source.name}” da base de conhecimento?`))
      return;
    setBusy(true);
    try {
      await request(`/api/react/knowledge/${source.id}`, { method: "DELETE" });
      setError(null);
      await load();
    } catch (removeError) {
      setError(
        removeError instanceof Error
          ? removeError.message
          : "Não foi possível remover a fonte.",
      );
    } finally {
      setBusy(false);
    }
  };

  const usagePercent = data?.limits.max_total_chars
    ? Math.min(
        100,
        Math.round(
          ((data.usage.chars || 0) / data.limits.max_total_chars) * 100,
        ),
      )
    : 0;

  return (
    <div className="agent-tab-body">
      {onOpenKnowledge && (
        <div className="entity-cross-links">
          <span>Gerencie todas as fontes (texto, site, PDF e áudio) em uma tela dedicada.</span>
          <button
            type="button"
            disabled={!agentId}
            onClick={() => { if (agentId) onOpenKnowledge(agentId); }}
          >
            Gerenciar base de conhecimento deste agente →
          </button>
        </div>
      )}
      <div className="agent-field-grid">
        <label className="agent-field agent-field-wide">
          <span>Conhecimento manual</span>
          <textarea
            rows={5}
            value={draft.knowledge}
            onChange={(event) => onChange("knowledge", event.target.value)}
            placeholder="Políticas, produtos, horários e respostas prontas."
          />
          <Help>
            Texto fixo que entra no contexto do agente. Salvo junto com o
            agente.
          </Help>
        </label>
        <label className="agent-field agent-field-wide">
          <span>Sites para buscar conhecimento</span>
          <textarea
            rows={3}
            value={draft.knowledge_links}
            onChange={(event) =>
              onChange("knowledge_links", event.target.value)
            }
            placeholder="Uma URL por linha · https://suaempresa.com.br"
          />
          <Help>
            Uma URL por linha. O conteúdo é baixado e salvo com o agente.
          </Help>
        </label>
      </div>

      <div className="agent-knowledge-block">
        <header>
          <div>
            <strong>Base de conhecimento (RAG)</strong>
            <Help>
              Fontes indexadas por agente. A IA busca só os trechos relevantes
              na hora da resposta.
            </Help>
          </div>
          {data && (
            <span className="agent-usage">
              {data.usage.sources}/{data.limits.max_sources_per_agent} fontes ·{" "}
              {data.usage.chars.toLocaleString("pt-BR")} caracteres
            </span>
          )}
        </header>
        {data && (
          <div className="agent-progress">
            <i style={{ width: `${usagePercent}%` }} />
          </div>
        )}
        {!agentId ? (
          <EmptyNote>
            Salve o agente antes de adicionar fontes à base.
          </EmptyNote>
        ) : (
          <>
            <div className="agent-knowledge-add">
              <div className="agent-kind-toggle">
                <button
                  type="button"
                  className={kind === "text" ? "is-active" : ""}
                  onClick={() => setKind("text")}
                >
                  Texto
                </button>
                <button
                  type="button"
                  className={kind === "site" ? "is-active" : ""}
                  onClick={() => setKind("site")}
                >
                  Site
                </button>
              </div>
              {kind === "text" ? (
                <textarea
                  rows={3}
                  value={text}
                  onChange={(event) => setText(event.target.value)}
                  placeholder="Cole aqui o conteúdo que a IA deve consultar."
                />
              ) : (
                <input
                  value={url}
                  onChange={(event) => setUrl(event.target.value)}
                  placeholder="https://suaempresa.com.br/duvidas"
                />
              )}
              <button
                className="secondary-button"
                type="button"
                disabled={busy}
                onClick={() => void add()}
              >
                + Adicionar fonte
              </button>
            </div>
            {error && <div className="agent-inline-error">{error}</div>}
            {data?.sources.length ? (
              <ul className="agent-source-list">
                {data.sources.map((source) => (
                  <li key={source.id}>
                    <span className="agent-source-kind">{source.kind}</span>
                    <span className="agent-source-name">{source.name}</span>
                    <span className="agent-source-chars">
                      {source.content_chars.toLocaleString("pt-BR")} car.
                    </span>
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => void remove(source)}
                    >
                      Remover
                    </button>
                  </li>
                ))}
              </ul>
            ) : (
              <EmptyNote>
                Nenhuma fonte indexada. Adicione texto ou site acima.
              </EmptyNote>
            )}
          </>
        )}
      </div>
    </div>
  );
}

export function AgentChannelsTab({
  agentId,
  options,
  onOpenChannels,
}: {
  agentId: number | null;
  options: Options;
  onOpenChannels?: () => void;
}) {
  const channels =
    agentId === null
      ? []
      : options.channels.filter((channel) => channel.agent_id === agentId);
  return (
    <div className="agent-tab-body">
      <p className="agent-tab-lead">
        Quem escolhe o agente é o canal. Aqui você só confere onde este agente
        já atende; para vincular ou trocar, edite o canal e selecione o agente
        que responde.
      </p>
      {onOpenChannels && (
        <div className="entity-cross-links">
          <span>
            Precisa conectar um número ou escolher o agente de um canal?
          </span>
          <button type="button" onClick={onOpenChannels}>
            Gerenciar canais →
          </button>
        </div>
      )}
      {agentId === null ? (
        <EmptyNote>Salve o agente para ver em quais canais ele atende.</EmptyNote>
      ) : channels.length ? (
        <ul className="agent-channel-readonly">
          {channels.map((channel) => (
            <li key={channel.id}>
              <span className="agent-source-kind">
                {channelLabel(channel.type)}
              </span>
              <span className="agent-source-name">{channel.name}</span>
              <span className="agent-source-chars">
                {channel.active ? "ativo" : "pausado"}
              </span>
            </li>
          ))}
        </ul>
      ) : (
        <EmptyNote>
          Este agente ainda não atende nenhum canal. Vincule-o no editor do
          canal, no modo IA.
        </EmptyNote>
      )}
    </div>
  );
}

export function AgentAdvancedTab({ draft, onChange, options }: TabProps) {
  return (
    <div className="agent-tab-body">
      <p className="agent-tab-lead">
        Campos menos usados. Deixe em branco se não precisar.
      </p>
      <label className="agent-field">
        <span>Menu de opções no 1º contato</span>
        <textarea
          rows={4}
          value={draft.menu_opcoes}
          onChange={(event) => onChange("menu_opcoes", event.target.value)}
          placeholder="Uma opção por linha. Ex.: Quanto custa?; Como funciona?; Falar com atendente."
        />
        <Help>
          Mostra um menu numerado no primeiro contato. Vazio desliga o menu.
        </Help>
      </label>
      <div className="agent-field">
        <span>Setores permitidos</span>
        <Help>
          Setores para onde a IA pode encaminhar quando o cliente pedir humano.
        </Help>
        {options.sectors.length ? (
          <div className="agent-inline-choices">
            {options.sectors.map((sector) => (
              <label key={sector.id} className="agent-inline-choice">
                <input
                  type="checkbox"
                  checked={draft.setor_ids.includes(sector.id)}
                  onChange={(event) =>
                    onChange(
                      "setor_ids",
                      event.target.checked
                        ? toggleNumber(draft.setor_ids, sector.id)
                        : draft.setor_ids.filter((item) => item !== sector.id),
                    )
                  }
                />
                <span>{sector.name}</span>
              </label>
            ))}
          </div>
        ) : (
          <EmptyNote>Nenhum setor ativo cadastrado.</EmptyNote>
        )}
      </div>
    </div>
  );
}
