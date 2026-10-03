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
  // Jellyfin-Umbau (docs/umbau-jellyfin.md): Menü, Bibliotheken, Dashboard, Aktionen
  menue: 'M4 6.5h16M4 12h16M4 17.5h16',
  herz: 'M12 19.5l-7.2-7.1a4.3 4.3 0 0 1 6.1-6.1l1.1 1.1 1.1-1.1a4.3 4.3 0 0 1 6.1 6.1z',
  musik: 'M9 17.5V5.5l10-2v12M9 17.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0zM19 15.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z',
  live: 'M3.5 6.5h17v11h-17zM8 20.5h8M10.5 9.5v5l4-2.5z',
  sammlung: 'M4 7.5h13v12.5H4zM7 4.5h13V17',
  liste: 'M4 6h11M4 11h11M4 16h6M14 14v6l5-3z',
  dashboard: 'M4 4h7v9H4zM13 4h7v5h-7zM13 11h7v9h-7zM4 15h7v5H4z',
  bearbeiten: 'M4 20h4.5L19.5 9 15 4.5 4 15.5zM12.5 7l4.5 4.5',
  abmelden: 'M10 4H4.5v16H10M15 7.5l4.5 4.5-4.5 4.5M19.5 12H9',
  gemeinsam: 'M9 11a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM3.5 19.5a5.5 5.5 0 0 1 11 0M15.5 5.2a3 3 0 0 1 0 5.6M17 14.2a5.5 5.5 0 0 1 3.5 5.3',
  cast: 'M3.5 9V5.5h17v13H14M3.5 12.5a6 6 0 0 1 6 6M3.5 15.5a3 3 0 0 1 3 3M3.5 18.5h.5',
  einladung: 'M3.5 6h17v12h-17zM3.5 6l8.5 7 8.5-7',
  bibliothek: 'M3.5 6h6l2 2.5h9v10h-17z',
  netzwerk: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM3 12h18M12 3c2.4 2.5 3.5 5.5 3.5 9s-1.1 6.5-3.5 9c-2.4-2.5-3.5-5.5-3.5-9s1.1-6.5 3.5-9z',
  aufgaben: 'M4.5 5.5h15v14.5h-15zM8.5 3v4M15.5 3v4M8.5 13.5l2.5 2.5 4.5-5',
  sicherung: 'M12 3.5l7.5 3v5.5c0 4.5-3.2 7.7-7.5 9-4.3-1.3-7.5-4.5-7.5-9V6.5zM8.5 12l2.5 2.5 4.5-5',
  protokoll: 'M6 3.5h8.5l4 4v13H6zM9 11.5h6.5M9 15h6.5M9 18.5h4',
  zufall: 'M4 7.5h3l9 9h4M4 16.5h3l2.5-2.5M13.5 10L16 7.5h4M17.5 5l2.5 2.5-2.5 2.5M17.5 14l2.5 2.5-2.5 2.5',
  warteschlange: 'M4 6h16M4 11h16M4 16h9M17 14.5v5.5M14.5 17h5',
  sortieren: 'M7.5 4v16M4 7.5L7.5 4 11 7.5M16.5 20V4M13 16.5l3.5 3.5 3.5-3.5',
  raster: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  download: 'M12 4v11M7 10l5 5 5-5M5 20h14',
  hochladen: 'M12 15V4M7 9l5-5 5 5M5 20h14',
  statistik: 'M4 20.5h16M6.5 17v-5M10.5 17V7M14.5 17v-7M18.5 17V4.5',
  link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
  loeschen: 'M4 7h16M9 7V4h6v3M6.5 7l1 13h9l1-13',
  bild: 'M4 5h16v14H4zM4 16l5-5 4 4 2.5-2.5L20 17M15.5 8.5h1v1h-1z',
  stern: 'M12 3.5l2.6 5.3 5.9.9-4.3 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.3-4.1 5.9-.9z',
  kalender: 'M4 5.5h16v14.5H4zM4 10h16M8 3v4M16 3v4',
  uhr: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM12 7v5.5l3.5 2',
  naechste: 'M18 5v14M6 5.5l9 6.5-9 6.5z',
  schluessel: 'M8 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM11.5 12H20M17.5 12v3M14.5 12v2',
  wechseln: 'M4 11.5V8h14.5l-3-3M20 12.5V16H5.5l3 3',
  server: 'M4 4.5h16v6H4zM4 13.5h16v6H4zM7.5 7.5h1M7.5 16.5h1',
  fernzugriff: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18zM8 12h8M13 8.5l3.5 3.5-3.5 3.5',
  puls: 'M3 12h4l2.5-6 5 12 2.5-6h4',
  stopp: 'M7 7h10v10H7z',
  punkt: 'M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6z',
  // Player (web-player)
  vorherige: 'M6 5v14M18 5.5L9 12l9 6.5z',
  vorspulen: 'M20 12a8 8 0 1 1-2.5-5.8M20 4v4.5h-4.5',
  vollbild: 'M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5',
  // Dashboard (web-admin)
  handy: 'M7 3.5h10v17H7zM11 17.5h2',
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
