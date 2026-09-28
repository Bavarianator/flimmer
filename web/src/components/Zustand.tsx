import type { ComponentChildren } from 'preact'
import { useEffect } from 'preact/hooks'
import { Gruppe, fokusBald } from '../lib/focus'
import { t } from '../lib/i18n'
import { Button } from './Button'
import { Icon, type IconName } from './Icon'

// Leer- und Fehlerzustand: sagt, was los ist und was hilft.
export function Leer(p: { titel: string; text?: string; icon?: IconName; aktion?: { label: string; onPress: () => void }; children?: ComponentChildren }) {
  useEffect(() => {
    if (p.aktion) fokusBald('zustand-aktion')
  }, [])
  return (
    <div class="zustand-platz">
      <div class="fl-zustand" role="status">
        {p.icon && (
          <div class="symbol">
            <Icon name={p.icon} />
          </div>
        )}
        <h3>{p.titel}</h3>
        {p.text && <p>{p.text}</p>}
        {p.aktion && (
          <Gruppe fokusKey="zustand">
            <Button fokusKey="zustand-aktion" variante="primaer" onPress={p.aktion.onPress}>
              {p.aktion.label}
            </Button>
          </Gruppe>
        )}
        {p.children}
      </div>
    </div>
  )
}

export function Fehler({ fehler, nochmal }: { fehler: string; nochmal?: () => void }) {
  useEffect(() => {
    if (nochmal) fokusBald('zustand-aktion')
  }, [])
  return (
    <div class="zustand-platz">
      <div class="fl-zustand fehler" role="alert">
        <div class="symbol">
          <Icon name="fehler" />
        </div>
        <h3>{t('fehler.titel')}</h3>
        <p>{t('fehler.text', { grund: fehler })}</p>
        {nochmal && (
          <Gruppe fokusKey="zustand">
            <Button fokusKey="zustand-aktion" variante="primaer" icon="neustart" onPress={nochmal}>
              {t('knopf.nochmal')}
            </Button>
          </Gruppe>
        )}
      </div>
    </div>
  )
}
