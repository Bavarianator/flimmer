// Hash-Routing: funktioniert auf jedem TV-Browser und in App-Hüllen ohne Server-Rewrites.
export function go(path: string) {
  location.hash = '#' + path
}

export function back() {
  if (history.length > 1) history.back()
  else go('/')
}

export function current(): string[] {
  return location.hash.replace(/^#\/?/, '').split('/').map(decodeURIComponent)
}
