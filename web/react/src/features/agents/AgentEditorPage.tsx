import { useCallback, useEffect, useState } from "react";
import {
  AgentAdvancedTab,
  AgentBehaviorTab,
  AgentChannelsTab,
  AgentKnowledgeTab,
  AgentSkillsTab,
  AgentToolsTab,
} from "./AgentTabs";
import {
  draftFromAgent,
  draftToBody,
  emptyDraft,
  emptyOptions,
  request,
  type Agent,
  type AgentResponse,
  type Draft,
  type Options,
} from "./agentShared";
import "./Agents.css";

type TabKey =
  | "comportamento"
  | "conhecimento"
  | "ferramentas"
  | "skills"
  | "canais"
  | "avancado";

const tabs: { key: TabKey; label: string; hint: string }[] = [
  { key: "comportamento", label: "Comportamento", hint: "Persona e IA" },
  { key: "conhecimento", label: "Conhecimento", hint: "O que a IA sabe" },
  { key: "ferramentas", label: "Ferramentas", hint: "Ações e integrações" },
  { key: "skills", label: "Skills", hint: "Instruções sob demanda" },
  { key: "canais", label: "Canais", hint: "Onde atende" },
  { key: "avancado", label: "Configurações", hint: "Menu e setores" },
];

export function AgentEditorPage({
  agentId,
  onBack,
  onSaved,
  onOpenKnowledge,
  onOpenProviders,
  onOpenSkills,
  onOpenChannels,
}: {
  agentId: number | null;
  onBack: () => void;
  onSaved: (id: number) => void;
  onOpenKnowledge?: (id: number) => void;
  onOpenProviders?: () => void;
  onOpenSkills?: () => void;
  onOpenChannels?: () => void;
}) {
  const [draft, setDraft] = useState<Draft | null>(null);
  const [options, setOptions] = useState<Options>(emptyOptions);
  const [tab, setTab] = useState<TabKey>("comportamento");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await request<AgentResponse>("/api/react/agents?limit=100");
      const items = payload.items || [];
      const options = payload.options || emptyOptions();
      setOptions(options);
      if (agentId === null) {
        setDraft(emptyDraft());
        setError(null);
        return;
      }
      const agent = items.find((item) => item.id === agentId);
      if (!agent) {
        setDraft(null);
        setError("Agente não encontrado.");
        return;
      }
      setDraft(draftFromAgent(agent));
      setError(null);
    } catch (loadError) {
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Não foi possível carregar o agente.",
      );
    } finally {
      setLoading(false);
    }
  }, [agentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const updateDraft = <K extends keyof Draft>(key: K, value: Draft[K]) =>
    setDraft((current) => (current ? { ...current, [key]: value } : current));

  const save = async () => {
    if (!draft) return;
    if (!draft.name.trim()) {
      setError("Informe um nome para o agente.");
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const url =
        agentId === null ? "/api/react/agents" : `/api/react/agents/${agentId}`;
      const saved = await request<Agent>(url, {
        method: agentId === null ? "POST" : "PATCH",
        body: JSON.stringify(draftToBody(draft)),
      });
      const targetId = agentId ?? saved.id;
      setNotice("Agente salvo com sucesso.");
      if (agentId === null) onSaved(targetId);
      else await load();
    } catch (saveError) {
      setError(
        saveError instanceof Error
          ? saveError.message
          : "Não foi possível salvar o agente.",
      );
    } finally {
      setSaving(false);
    }
  };

  const title =
    draft?.name?.trim() || (agentId === null ? "Novo agente" : "Agente");

  return (
    <div className="agent-screen">
      <header className="agent-screen-header">
        <button
          className="agent-back"
          type="button"
          onClick={onBack}
          aria-label="Voltar para a lista de agentes"
        >
          ←
        </button>
        <div className="agent-screen-title">
          <span className="eyebrow">
            {agentId === null ? "Novo agente" : "Editar agente"}
          </span>
          <h1>{title}</h1>
        </div>
        <div className="agent-screen-actions">
          <button className="secondary-button" type="button" onClick={onBack}>
            Voltar
          </button>
          <button
            className="primary-button"
            type="button"
            disabled={saving || loading || !draft}
            onClick={() => void save()}
          >
            {saving ? "Salvando…" : "Salvar agente"}
          </button>
        </div>
      </header>

      {error && (
        <div className="agents-alert" role="alert">
          <span>!</span>
          {error}
          <button
            type="button"
            onClick={() => setError(null)}
            aria-label="Fechar aviso"
          >
            ×
          </button>
        </div>
      )}
      {notice && (
        <div className="agent-notice" role="status">
          <span>✓</span>
          {notice}
          <button
            type="button"
            onClick={() => setNotice(null)}
            aria-label="Fechar aviso"
          >
            ×
          </button>
        </div>
      )}

      {loading ? (
        <div className="agents-state">
          <span className="agents-spinner" /> Carregando agente…
        </div>
      ) : !draft ? (
        <div className="agents-empty">
          <strong>Agente não encontrado</strong>
          <p>Ele pode ter sido excluído. Volte para a lista de agentes.</p>
          <button className="primary-button" type="button" onClick={onBack}>
            Voltar para agentes
          </button>
        </div>
      ) : (
        <div className="agent-tabs-card">
          <nav className="agent-tabs" role="tablist" aria-label="Seções do agente">
            {tabs.map((item) => (
              <button
                key={item.key}
                type="button"
                role="tab"
                aria-selected={tab === item.key}
                className={tab === item.key ? "is-active" : ""}
                onClick={() => setTab(item.key)}
              >
                <strong>{item.label}</strong>
                <small>{item.hint}</small>
              </button>
            ))}
          </nav>
          <section className="agent-tab-panel" role="tabpanel">
            {tab === "comportamento" && (
              <AgentBehaviorTab
                draft={draft}
                onChange={updateDraft}
                options={options}
                onOpenProviders={onOpenProviders}
              />
            )}
            {tab === "conhecimento" && (
              <AgentKnowledgeTab
                draft={draft}
                onChange={updateDraft}
                agentId={agentId}
                onOpenKnowledge={onOpenKnowledge}
              />
            )}
            {tab === "ferramentas" && (
              <AgentToolsTab
                draft={draft}
                onChange={updateDraft}
                options={options}
              />
            )}
            {tab === "skills" && (
              <AgentSkillsTab agentId={agentId} onOpenSkills={onOpenSkills} />
            )}
            {tab === "canais" && (
              <AgentChannelsTab
                agentId={agentId}
                options={options}
                onOpenChannels={onOpenChannels}
              />
            )}
            {tab === "avancado" && (
              <AgentAdvancedTab
                draft={draft}
                onChange={updateDraft}
                options={options}
              />
            )}
          </section>
        </div>
      )}
    </div>
  );
}

export default AgentEditorPage;
