import { Gruppe, useFokus } from '../lib/focus'

export interface Tab {
  id: string
  label: string
}

function TabKnopf({ tab, an, onWahl, praefix }: { tab: Tab; an: boolean; onWahl: (id: string) => void; praefix: string }) {
  const f = useFokus<HTMLButtonElement>({ fokusKey: praefix + '-' + tab.id, onPress: () => onWahl(tab.id) })
  return (
    <button ref={f.ref} {...f.dom} type="button" role="tab" aria-selected={an} class={'fl-tab' + (an ? ' an' : '') + (f.fokus ? ' ist-fokus' : '')}>
      {tab.label}
    </button>
  )
}

// Text-Tabs mit Unterstrich für den aktiven Eintrag (Staffelwahl, Einstellungsbereiche).
export function Tabs(p: { tabs: Tab[]; aktiv: string; onWahl: (id: string) => void; fokusKey: string; label?: string }) {
  return (
    <Gruppe fokusKey={p.fokusKey} class="fl-tabs" bevorzugt={p.fokusKey + '-' + p.aktiv} label={p.label}>
      <div role="tablist">
        {p.tabs.map((x) => (
          <TabKnopf key={x.id} tab={x} an={x.id === p.aktiv} onWahl={p.onWahl} praefix={p.fokusKey} />
        ))}
      </div>
    </Gruppe>
  )
}
