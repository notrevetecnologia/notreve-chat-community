type AudioScope = Window & typeof globalThis & { webkitAudioContext?: typeof AudioContext }

let audioContext: AudioContext | null = null
let titleTimer: number | null = null
let titleBase = ''

const resolveAudioContext = (): AudioContext | null => {
  if (typeof window === 'undefined') return null
  if (audioContext) return audioContext
  const scope = window as AudioScope
  const Context = scope.AudioContext ?? scope.webkitAudioContext
  if (!Context) return null
  try {
    audioContext = new Context()
  } catch {
    audioContext = null
  }
  return audioContext
}

export function playNewMessageSound(): void {
  const context = resolveAudioContext()
  if (!context) return
  const emit = () => {
    const start = context.currentTime
    const oscillator = context.createOscillator()
    const gain = context.createGain()
    oscillator.type = 'sine'
    oscillator.frequency.setValueAtTime(880, start)
    oscillator.frequency.setValueAtTime(1318.51, start + 0.12)
    gain.gain.setValueAtTime(0.0001, start)
    gain.gain.exponentialRampToValueAtTime(0.16, start + 0.02)
    gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.34)
    oscillator.connect(gain)
    gain.connect(context.destination)
    oscillator.start(start)
    oscillator.stop(start + 0.36)
  }
  if (context.state === 'suspended') {
    void context.resume().then(emit).catch(() => undefined)
    return
  }
  emit()
}

export function notificationPermissionState(): NotificationPermission | 'unsupported' {
  if (typeof window === 'undefined' || !('Notification' in window)) return 'unsupported'
  return Notification.permission
}

export function requestNotificationPermission(): Promise<NotificationPermission> {
  if (typeof window === 'undefined' || !('Notification' in window)) return Promise.resolve('denied')
  try {
    return Notification.requestPermission()
  } catch {
    return Promise.resolve('denied')
  }
}

export function showDesktopNotification(title: string, body: string): void {
  if (typeof window === 'undefined' || !('Notification' in window)) return
  if (Notification.permission !== 'granted') return
  try {
    const notification = new Notification(title, { body, tag: 'notreve-inbox' })
    notification.onclick = () => {
      window.focus()
      notification.close()
    }
  } catch {
    return
  }
}

export function startTitleAlert(baseTitle: string, message = 'Nova mensagem'): void {
  if (typeof document === 'undefined') return
  titleBase = baseTitle.trim() || document.title
  if (titleTimer !== null) return
  let alternate = false
  titleTimer = window.setInterval(() => {
    document.title = alternate ? titleBase : `(${message}) ${titleBase}`
    alternate = !alternate
  }, 1200)
}

export function stopTitleAlert(): void {
  if (typeof document === 'undefined') return
  if (titleTimer !== null) {
    window.clearInterval(titleTimer)
    titleTimer = null
  }
  if (titleBase) document.title = titleBase
}
