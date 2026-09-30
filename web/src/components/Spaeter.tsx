import type { ComponentType } from 'preact'
import { useEffect, useState } from 'preact/hooks'

// Lädt eine Komponente erst beim ersten Rendern nach (wie spaeter() in main.tsx). So bleiben
// Bibliothek, Detailseiten und Kontextmenü aus dem Start-JS (Budget: 80 KB, legacy 120 KB gzip).
export function spaeter<P>(laden: () => Promise<ComponentType<P>>): ComponentType<P> {
  let K: ComponentType<P> | null = null
  return (p: P) => {
    const [, neu] = useState(0)
    useEffect(() => {
      if (!K) laden().then((k) => ((K = k), neu(1)))
    }, [])
    return K ? <K {...(p as P & object)} /> : null
  }
}
