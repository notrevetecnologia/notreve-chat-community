import { useCallback, useEffect, useState } from 'react'
import { Modal } from '../../shared/Modal'
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
  const [error, setError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    setNotice(null)
    setSetup(null)
    setCode('')
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

  return (
    <Modal open={open} onClose={onClose} title="Segurança da conta" size="sm">
      {loading ? <p className="security-state">Carregando…</p> : (
        <div className="security-body">
          {error && <div className="security-alert" role="alert">{error}</div>}
          {notice && <div className="security-notice" role="status">{notice}</div>}
          {status && (
            <>
              <p className="security-copy">A autenticação em duas etapas (2FA) pede um código do seu app autenticador a cada login.</p>
              <div className="security-status">
                <span className={`security-dot${status.enabled ? ' is-on' : ''}`} aria-hidden="true" />
                <strong>{status.enabled ? 'Ativada' : 'Desativada'}</strong>
                {status.required && <span className="security-tag">obrigatória para o super_admin</span>}
              </div>

              {!status.enabled && !setup && (
                <button className="primary-button" type="button" disabled={busy} onClick={() => void beginSetup()}>Ativar 2FA</button>
              )}

              {!status.enabled && setup && (
                <>
                  {setup.qr && <img className="security-qr" src={setup.qr} alt="QR Code para configurar o autenticador" width={208} height={208} />}
                  <div className="security-secret">
                    <span>Chave de configuração</span>
                    <code>{setup.secret}</code>
                  </div>
                  <p className="security-hint">Escaneie o QR Code no Google/Microsoft Authenticator ou adicione a chave acima. Depois informe o código gerado.</p>
                  <label className="security-field">
                    <span>Código de 6 dígitos</span>
                    <input inputMode="numeric" maxLength={6} value={code} onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))} placeholder="000000" />
                  </label>
                  <div className="security-actions">
                    <button className="secondary-button" type="button" disabled={busy} onClick={() => { setSetup(null); setCode(''); setError(null) }}>Cancelar</button>
                    <button className="primary-button" type="button" disabled={busy} onClick={() => void confirm()}>Confirmar e ativar</button>
                  </div>
                </>
              )}

              {status.enabled && !status.required && (
                <>
                  <p className="security-hint">Para desativar, confirme com o código atual do seu autenticador.</p>
                  <label className="security-field">
                    <span>Código de 6 dígitos</span>
                    <input inputMode="numeric" maxLength={6} value={code} onChange={(event) => setCode(event.target.value.replace(/\D/g, '').slice(0, 6))} placeholder="000000" />
                  </label>
                  <button className="secondary-button" type="button" disabled={busy} onClick={() => void disable()}>Desativar 2FA</button>
                </>
              )}

              {status.enabled && status.required && (
                <p className="security-hint">O super_admin mantém a 2FA sempre ativa por segurança da plataforma.</p>
              )}
            </>
          )}
        </div>
      )}
    </Modal>
  )
}

export default SecurityModal
