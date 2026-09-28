import { fortsetzenAb, type Item } from '../lib/api'
import { t, uhr } from '../lib/i18n'
import { go, pfad } from '../lib/router'
import { Button } from './Button'

// Abspielen bzw. Fortsetzen ab … und Von vorn. Fokus-Schlüssel: 'abspielen', 'vonvorn'.
export function Abspielknoepfe({ it, extra }: { it: Item; extra?: preact.ComponentChildren }) {
  const pos = fortsetzenAb(it.id)
  return (
    <>
      {pos > 0 ? (
        <>
          <Button fokusKey="abspielen" variante="primaer" icon="abspielen" onPress={() => go(pfad('watch', it.id))}>
            {t('knopf.fortsetzen', { zeit: uhr(pos) })}
          </Button>
          <Button fokusKey="vonvorn" icon="neustart" onPress={() => go(pfad('watch', it.id, 0))}>
            {t('knopf.vonvorn')}
          </Button>
        </>
      ) : (
        <Button fokusKey="abspielen" variante="primaer" icon="abspielen" onPress={() => go(pfad('watch', it.id))}>
          {t('knopf.abspielen')}
        </Button>
      )}
      {extra}
    </>
  )
}
