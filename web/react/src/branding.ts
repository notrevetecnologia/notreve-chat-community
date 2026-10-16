export type Branding = {
  name?: string
  logo_url?: string
  favicon_url?: string
  primary_color?: string
  secondary_color?: string
  accent_color?: string
  background_color?: string
  loading_color?: string
}

const defaultColors = {
  primary: '#12948e',
  secondary: '#071c1c',
  accent: '#d5f37a',
  background: '#f5faf8',
  loading: '#12948e',
}

export function applyBrandingTheme(branding: Branding | null | undefined): void {
  const root = document.documentElement
  const set = (name: string, value: string | undefined, fallback: string) => {
    root.style.setProperty(name, value?.trim() || fallback)
  }
  set('--brand-primary', branding?.primary_color, defaultColors.primary)
  set('--brand-secondary', branding?.secondary_color, defaultColors.secondary)
  set('--brand-accent', branding?.accent_color, defaultColors.accent)
  set('--shell-background', branding?.background_color, defaultColors.background)
  set('--brand-loading', branding?.loading_color, defaultColors.loading)
  const faviconURL = branding?.favicon_url?.trim()
  if (faviconURL) {
    const favicon = document.querySelector<HTMLLinkElement>('link[rel="icon"]') ?? document.createElement('link')
    favicon.rel = 'icon'
    favicon.href = faviconURL
    document.head.appendChild(favicon)
  }
}

export function brandingDocumentTitle(name?: string): void {
  const resolved = name?.trim()
  if (resolved) document.title = resolved
}
