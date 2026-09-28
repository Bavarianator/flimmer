// Linien-Icons auf 24er-Raster wie im Designsystem: 1,5 px Strich, eckige Enden (Stil in .fl-icon).
// Farbe über currentColor.
// Ohne label sind sie dekorativ (aria-hidden); allein stehende Icons brauchen ein label.
const pfade = {
  abspielen: 'M8 5.5v13a1 1 0 0 0 1.5.9l10.4-6.5a1 1 0 0 0 0-1.8L9.5 4.6A1 1 0 0 0 8 5.5z',
  pause: 'M8 5v14M16 5v14',
  zurueck: 'M15 5l-7 7 7 7',
  weiter: 'M9 5l7 7-7 7',
  hoch: 'M5 15l7-7 7 7',
  runter: 'M5 9l7 7 7-7',
  suche: 'M11 17.5a6.5 6.5 0 1 0 0-13 6.5 6.5 0 0 0 0 13zM16 16l4.5 4.5',
  filter: 'M4 6h16M7 12h10M10 18h4',
  plus: 'M12 5v14M5 12h14',
  schloss: 'M7 10.5h10a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-6a2 2 0 0 1 2-2zM8 10.5V8a4 4 0 0 1 8 0v2.5',
  einstellungen: 'M4 7h10M18 7h2M4 17h2M10 17h10M16 4.5v5M8 14.5v5',
  info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 11v6M12 7.5v.5',
  haken: 'M5 12.5l4.5 4.5L19 7.5',
  schliessen: 'M6 6l12 12M18 6L6 18',
  neustart: 'M4 12a8 8 0 1 0 2.5-5.8M4 4v4.5h4.5',
  fernseher: 'M3.5 5.5h17v11h-17zM8 20h8',
  profil: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4.5 20a7.5 7.5 0 0 1 15 0',
  start: 'M4 11l8-7 8 7M6 9.5V20h12V9.5',
  film: 'M4 5h16v14H4zM8 5v14M16 5v14M4 9.5h4M4 14.5h4M16 9.5h4M16 14.5h4',
  serie: 'M4 7h16v12H4zM8 3.5l4 3.5 4-3.5',
  ton: 'M4 9.5v5h4l5 4v-13l-5 4zM17 9a4 4 0 0 1 0 6',
  untertitel: 'M3.5 5.5h17v13h-17zM7 12.5h4M13 12.5h4M7 15.5h7',
  fehler: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7.5v6M12 16.5v.5',
  mehr: 'M5 12h.5M12 12h.5M19 12h.5',
}

export type IconName = keyof typeof pfade

export function Icon({ name, label, class: cls }: { name: IconName; label?: string; class?: string }) {
  return (
    <svg
      class={'fl-icon' + (cls ? ' ' + cls : '')}
      viewBox="0 0 24 24"
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : 'true'}
      focusable="false"
    >
      <path d={pfade[name]} />
    </svg>
  )
}
