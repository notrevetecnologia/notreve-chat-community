import { useCallback, useEffect, useRef, useState } from "react";
import {
  emptyOptions,
  request,
  requestEnvelope,
  type Agent,
  type AgentResponse,
  type Options,
} from "./agentShared";
import { Icon } from '../../shared/icons'
import { FilterBar, FilterSearch, FilterSelect, Pagination, DEFAULT_PAGE_SIZE, encodeCursor } from "../../shared/Pagination";
import "./Agents.css";

const isAbortError = (error: unknown): boolean =>
  typeof error === "object" && error !== null && "name" in error && error.name === "AbortError";

export function AgentsPage({
  onBack,
  onOpenAgent,
}: {
  onBack: () => void;
  onOpenAgent: (id: number | null) => void;
}) {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [options, setOptions] = useState<Options>(emptyOptions);
  const [loading, setLoading] = useState(true);
  const [actionID, setActionID] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [status, setStatus] = useState("");
  const [offset, setOffset] = useState(0);
  const [total, setTotal] = useState(0);
  const loadRequestRef = useRef(0);

  const load = useCallback(async (signal?: AbortSignal) => {
    const requestID = ++loadRequestRef.current;
    setLoading(true);
    try {
      const params = new URLSearchParams({ limit: String(DEFAULT_PAGE_SIZE) });
      if (debouncedQuery.trim()) params.set("q", debouncedQuery.trim());
      if (status) params.set("status", status);
      if (offset > 0) params.set("cursor", encodeCursor(offset));
      const payload = await requestEnvelope<AgentResponse>(`/api/react/agents?${params.toString()}`, { signal });
      if (signal?.aborted || requestID !== loadRequestRef.current) return;
      setAgents(payload.data.items || []);
      setOptions(payload.data.options || emptyOptions());
      setTotal(payload.meta.total ?? payload.data.items?.length ?? 0);
      setError(null);
    } catch (loadError) {
      if (isAbortError(loadError) || requestID !== loadRequestRef.current) return;
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Não foi possível carregar os agentes.",
      );
    } finally {
      if (requestID === loadRequestRef.current) setLoading(false);
    }
  }, [debouncedQuery, status, offset]);

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedQuery(query), 275);
    return () => window.clearTimeout(timer);
  }, [query]);
  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, [load]);

  const toggle = async (agent: Agent) => {
    await runAction(agent.id, () =>
      request(`/api/react/agents/${agent.id}/toggle`, {
        method: "POST",
        body: "{}",
      }),
    );
  };
  const refetch = async (agent: Agent) => {
    await runAction(agent.id, () =>
      request(`/api/react/agents/${agent.id}/refetch`, {
        method: "POST",
        body: "{}",
      }),
    );
  };
  const remove = async (agent: Agent) => {
    if (!window.confirm(`Excluir o agente “${agent.name}”?`)) return;
    await runAction(agent.id, () =>
      request(`/api/react/agents/${agent.id}`, { method: "DELETE" }),
    );
  };
  const runAction = async (id: number, action: () => Promise<unknown>) => {
    setActionID(id);
    try {
      await action();
      await load();
    } catch (actionError) {
      setError(
        actionError instanceof Error
          ? actionError.message
          : "Não foi possível concluir a ação.",
      );
    } finally {
      setActionID(null);
    }
  };

  const stop = (event: { stopPropagation: () => void }) =>
    event.stopPropagation();

  return (
    <div className="agents-page">
      <section className="agents-intro">
        <div>
          <span className="eyebrow">Inteligência da operação</span>
          <h2>Agentes que conhecem seu negócio.</h2>
          <p>
            Crie assistentes com instruções, fontes e tools isoladas para a sua
            empresa. É o canal que escolhe qual agente atende.
          </p>
          <ol className="agents-steps">
            <li>1. Crie um provedor de IA</li>
            <li>2. Crie o agente (persona)</li>
            <li>3. Conecte o canal e escolha qual agente atende</li>
          </ol>
        </div>
        <div className="agents-intro-actions">
          <button className="secondary-button" type="button" onClick={onBack}>
            <Icon name="arrow-left" /> Visão geral
          </button>
          <button
            className="primary-button"
            type="button"
            onClick={() => onOpenAgent(null)}
          >
            + Novo agente
          </button>
        </div>
      </section>
      {error && (
        <div className="agents-alert" role="alert">
          <span><Icon name="alert-circle" /></span>
          {error}
          <button
            type="button"
            onClick={() => setError(null)}
            aria-label="Fechar aviso"
          >
            <Icon name="close" />
          </button>
        </div>
      )}
      <FilterBar
        resultLabel={`${total} agente${total === 1 ? "" : "s"}`}
        onReset={() => { setOffset(0); setQuery(""); setStatus(""); }}
      >
        <FilterSearch value={query} onChange={(value) => { setOffset(0); setQuery(value); }} placeholder="Buscar agente, instrução ou provedor" />
        <FilterSelect value={status} onChange={(value) => { setOffset(0); setStatus(value); }} label="Filtrar status" options={[{ value: "", label: "Todos os status" }, { value: "ativos", label: "Ativos" }, { value: "inativos", label: "Pausados" }]} />
      </FilterBar>
      {loading ? (
        <div className="agents-state">
          <span className="agents-spinner" /> Carregando agentes…
        </div>
      ) : agents.length === 0 ? (
        <div className="agents-empty">
          <div className="agents-empty-icon"><Icon name="agents" /></div>
          <strong>Nenhum agente criado</strong>
          <p>
            Comece com um assistente para responder automaticamente nos seus
            canais.
          </p>
          <button
            className="primary-button"
            type="button"
            onClick={() => onOpenAgent(null)}
          >
            Criar primeiro agente
          </button>
        </div>
      ) : (
        <div className="agents-grid">
          {agents.map((agent) => (
            <article
              className="agent-card"
              key={agent.id}
            >
              <button className="agent-card-open" type="button" onClick={() => onOpenAgent(agent.id)} aria-label={`Editar agente ${agent.name}`} />
              <header>
                <div
                  className={`agent-avatar ${agent.active ? "is-active" : ""}`}
                >
                  <Icon name="bot" />
                </div>
                <div>
                  <span className="agent-kicker">Assistente virtual</span>
                  <h3>{agent.name}</h3>
                </div>
                <span
                  className={`agent-status ${agent.active ? "is-active" : ""}`}
                >
                  <i />
                  {agent.active ? "Ativo" : "Pausado"}
                </span>
              </header>
              <p className="agent-prompt">
                {agent.system_prompt || "Sem instrução inicial definida."}
              </p>
              <div className="agent-metadata">
                <span><Icon name="providers" /> {agent.provider_name || "Provedor padrão"}</span>
                <span>
                  <Icon name="knowledge" /> {agent.knowledge_chars.toLocaleString("pt-BR")} caracteres
                </span>
                <span>
                  <Icon name="link" /> {agent.knowledge_links.length} fonte
                  {agent.knowledge_links.length === 1 ? "" : "s"}
                </span>
              </div>
              <div className="agent-chips">
                {agent.tool_ids.length > 0 && (
                  <span>
                    {agent.tool_ids.length} tool
                    {agent.tool_ids.length === 1 ? "" : "s"}
                  </span>
                )}
              </div>
              <div className="agent-channels">
                <span>Atende em</span>
                <strong>
                  {options.channels
                    .filter((channel) => channel.agent_id === agent.id)
                    .map((channel) => channel.name)
                    .join(", ") || "Nenhum canal vinculado"}
                </strong>
              </div>
              <footer>
                <button
                  type="button"
                  onClick={(event) => {
                    stop(event);
                    onOpenAgent(agent.id);
                  }}
                >
                  Editar
                </button>
                <button
                  type="button"
                  disabled={actionID === agent.id}
                  onClick={(event) => {
                    stop(event);
                    void toggle(agent);
                  }}
                >
                  {agent.active ? "Pausar" : "Ativar"}
                </button>
                <button
                  type="button"
                  disabled={actionID === agent.id}
                  onClick={(event) => {
                    stop(event);
                    void refetch(agent);
                  }}
                >
                  <Icon name="refresh" /> Fontes
                </button>
                <button
                  className="is-danger"
                  type="button"
                  disabled={actionID === agent.id}
                  onClick={(event) => {
                    stop(event);
                    void remove(agent);
                  }}
                >
                  Excluir
                </button>
              </footer>
            </article>
          ))}
        </div>
      )}
      {!loading && agents.length > 0 && (
        <Pagination total={total} limit={DEFAULT_PAGE_SIZE} offset={offset} onOffsetChange={setOffset} loading={loading} />
      )}
    </div>
  );
}

export default AgentsPage;
