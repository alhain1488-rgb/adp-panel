import { Button } from '@/components/ui/button'
import { useLang, useT } from '@/i18n/i18n'

// Toggles the UI language between Russian and English. Shows the current code.
export function LanguageToggle() {
  const { lang, setLang } = useLang()
  const t = useT()
  return (
    <Button
      variant="ghost"
      size="icon"
      aria-label="Language"
      title={lang === 'ru' ? t('lang.switchToEn') : t('lang.switchToRu')}
      onClick={() => setLang(lang === 'ru' ? 'en' : 'ru')}
    >
      <span className="text-xs font-semibold tracking-wide">{lang === 'ru' ? 'RU' : 'EN'}</span>
    </Button>
  )
}
