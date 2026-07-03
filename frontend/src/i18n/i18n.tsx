// Lightweight i18n: a language context + a useT() hook over a central RU/EN
// dictionary. Russian is the default; the choice is persisted in localStorage.
// No external dependency — the whole surface is a dictionary lookup with {var}
// interpolation and English (then the key) as fallback.
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { dict } from './translations'
import { formatRelativeTime } from '@/lib/utils'

export type Lang = 'ru' | 'en'

const STORAGE_KEY = 'adp_lang'

function initialLang(): Lang {
  try {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved === 'en' || saved === 'ru') return saved
  } catch {
    /* ignore */
  }
  return 'ru'
}

type LangContextValue = { lang: Lang; setLang: (l: Lang) => void }

const LangContext = createContext<LangContextValue>({ lang: 'ru', setLang: () => {} })

export function LanguageProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(initialLang)

  const setLang = useCallback((l: Lang) => {
    setLangState(l)
    try {
      localStorage.setItem(STORAGE_KEY, l)
    } catch {
      /* ignore */
    }
    if (typeof document !== 'undefined') document.documentElement.lang = l
  }, [])

  // Reflect the active language on <html lang> for a11y and correct hyphenation.
  useEffect(() => {
    if (typeof document !== 'undefined') document.documentElement.lang = lang
  }, [lang])

  const value = useMemo(() => ({ lang, setLang }), [lang, setLang])
  return <LangContext.Provider value={value}>{children}</LangContext.Provider>
}

export function useLang(): LangContextValue {
  return useContext(LangContext)
}

// useRelTime returns a relative-time formatter bound to the current language.
export function useRelTime() {
  const { lang } = useContext(LangContext)
  return useCallback((iso?: string | null) => formatRelativeTime(iso, lang), [lang])
}

// t(key, vars?) — returns the translation for the current language, falling back
// to English and then the key. Supports "{name}" interpolation.
export function useT() {
  const { lang } = useContext(LangContext)
  return useCallback(
    (key: string, vars?: Record<string, string | number>): string => {
      const entry = (dict as Record<string, { en: string; ru: string }>)[key]
      let s = entry ? entry[lang] ?? entry.en : key
      if (vars) {
        for (const [k, v] of Object.entries(vars)) s = s.split(`{${k}}`).join(String(v))
      }
      return s
    },
    [lang],
  )
}
