import { useCallback, useEffect, useState } from 'react'
import { Modal } from '../../shared/Modal'
import { Icon } from '../../shared/icons'
import { confirmMFA, disableMFA, loadMFAStatus, startMFASetup, type MFAStatus } from './securityApi'
import './Security.css'

type SecurityModalProps = {
  open: boolean
  onClose: () => void
}

export function SecurityModal({ open, onClose }: SecurityModalProps) {
  const [status, setStatus] = useState<MFAStatus | null>(null)
  const [setup, setSetup] = useState<{ secret: string; uri: string; qr: string } | null>(null)
  const [code, setCode] = useState('')
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [copied, setCopied] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    setNotice(null)
    setSetup(null)
    setCode('')
    setCopied(false)
    try {
      setStatus(await loadMFAStatus())
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : 'Não foi possível carregar a segurança da conta.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { if (open) void load() }, [open, load])

  const beginSetup = async () => {
    setBusy(true)
    setError(null)
    setNotice(null)
    try {
      const result = await startMFASetup()
      if (result.enabled) { await load(); return }
      setSetup({ secret: result.secret ?? '', uri: result.uri ?? '', qr: result.qr ?? '' })
    } catch (setupError) {
      setError(setupError instanceof Error ? setupError.message : 'Não foi possível iniciar a configuração.')
    } finally {
      setBusy(false)
    }
  }

  const confirm = async () => {
    if (code.length !== 6) { setError('Informe o código de 6 dígitos.'); return }
    setBusy(true)
    setError(null)
    try {
      await confirmMFA(code)
      setSetup(null)
      setCode('')
      setNotice('Autenticação em duas etapas ativada.')
      setStatus({ enabled: true, required: status?.required ?? false })
    } catch (confirmError) {
      setError(confirmError instanceof Error ? confirmError.message : 'Código inválido.')
    } finally {
      setBusy(false)
    }
  }

  const disable = async () => {
    if (code.length !== 6) { setError('Informe o código de 6 dígitos.'); return }
    setBusy(true)
    setError(null)
    try {
      await disableMFA(code)
      setCode('')
      setNotice('Autenticação em duas etapas desativada.')
      setStatus({ enabled: false, required: status?.required ?? false })
    } catch (disableError) {
      setError(disableError instanceof Error ? disableError.message : 'Não foi possível desativar.')
    } finally {
      setBusy(false)
    }
  }

  const copySecret = async () => {
    if (!setup?.secret) return
    try {
      await navigator.clipboard.writeText(setup.secret)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      setError('Não foi possível copiar a chave. Copie manualmente.')
    }
  }

  const closeSetup = () => {
    setSetup(null)
    setCode('')
    setError(null)
    setCopied(false)
  }

  return (
    <Modal open={open} onClose={onClose} title="Segurança da conta" size="sm">
      {loading ? <p className="security-state">Carregando…</p> : (
        <div className="security-body">
          {error && <div className="security-alert" role="alert">{error}</div>}
          {notice && <div className="security-notice" role="status">{notice}</div>}
          {status && (setup ? (
            <div className="security-step">
              <button className="security-back" type="button" onClick={closeSetup}>
                <Icon name="chevron-left" />
                Voltar
              </button>

              <div className="security-hero">
                <span className="security-hero-icon" aria-hidden="true"><Icon name="shield" /></span>
                <div>
                  <h3>Ative a autenticação em duas etapas</h3>
                  <p>Escaneie o QR Code ou digite a chave no seu app autenticador e informe o código gerado.</p>
                </div>
              </div>

              {setup.qr && <img className="security-qr" src={setup.qr} alt="QR Code para configurar o autenticador" width={200} height={200} />}

              <div className="security-secret">
                <span>Chave de configuração</span>
                <div className="security-secret-row">
                  <code>{setup.secret}</code>
                  <button className="security-copy" type="button" onClick={() => void copySecret()}>
                    <Icon name={copied ? 'check' : 'copy'} />
                    {copied ? 'Copiado' : 'Copiar'}
                  </button>
                </div>
              </div>

              <label className="security-field">
                <span>Código de 6 dígitos</span>
                <input
                  autoFocus
                  inputMode="numeric"
                  maxLength={6}
                  value={code}
                  onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
                  placeholder="000000"
                />
              </label>

              <div className="security-actions">
                <button className="secondary-button" type="button" disabled={busy} onClick={closeSetup}>Cancelar</button>
                <button className="primary-button" type="button" disabled={busy} onClick={() => void confirm()}>Ativar proteção</button>
              </div>
            </div>
          ) : (
            <>
              <div className="security-hero">
                <span className="security-hero-icon" aria-hidden="true"><Icon name="shield" /></span>
                <div>
                  <h3>Autenticação em duas etapas</h3>
                  <p>A 2FA pede um código do seu app autenticador a cada login, protegendo a conta mesmo se a senha vazar.</p>
                </div>
              </div>

              <div className="security-status">
                <span className={`security-dot${status.enabled ? ' is-on' : ''}`} aria-hidden="true" />
                <strong>{status.enabled ? 'Ativada' : 'Desativada'}</strong>
                {status.required && <span className="security-tag">obrigatória</span>}
              </div>

              {!status.enabled && (
                <button className="primary-button security-primary" type="button" disabled={busy} onClick={() => void beginSetup()}>
                  Ativar 2FA
                </button>
              )}

              {status.enabled && !status.required && (
                <div className="security-disable">
                  <p className="security-hint">Para desativar, confirme com o código atual do seu autenticador.</p>
                  <label className="security-field">
                    <span>Código de 6 dígitos</span>
                    <input
                      inputMode="numeric"
                      maxLength={6}
                      value={code}
                      onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))}
                      placeholder="000000"
                    />
                  </label>
                  <button className="security-danger" type="button" disabled={busy} onClick={() => void disable()}>
                    Desativar 2FA
                  </button>
                </div>
              )}

              {status.enabled && status.required && (
                <p className="security-hint">O super_admin mantém a 2FA sempre ativa por segurança da plataforma.</p>
              )}
            </>
          ))}
        </div>
      )}
    </Modal>
  )
}

export default SecurityModal
