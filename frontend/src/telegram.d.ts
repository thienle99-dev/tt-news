interface TelegramWebApp {
  initData: string
  colorScheme: 'light' | 'dark'
  themeParams: Record<string, string>
	initDataUnsafe?: { user?: { language_code?: string } }
  ready(): void
  expand(): void
  openLink(url: string): void
  openTelegramLink?(url: string): void
  onEvent(event: string, callback: () => void): void
  offEvent(event: string, callback: () => void): void
}
interface Window { Telegram?: { WebApp?: TelegramWebApp } }
