// Skeletons statt Spinner: feste Maße wie die echten Karten, damit nichts springt.
export function SkeletonKarten({ n = 6, breit }: { n?: number; breit?: boolean }) {
  const k = []
  for (let i = 0; i < n; i++)
    k.push(
      <div key={i} class={'fl-karte' + (breit ? ' breit' : '')} aria-hidden="true">
        <div class="rahmen fl-skel" />
        <div class="meta">
          <div class="fl-skel zeile" style={{ width: '70%' }} />
          <div class="fl-skel zeile" style={{ width: '45%' }} />
        </div>
      </div>,
    )
  return <>{k}</>
}

export function SkeletonReihe({ breit, n }: { breit?: boolean; n?: number }) {
  return (
    <section class="fl-reihe reihe" aria-busy="true">
      <div class="fl-skel zeile skel-titel" />
      <div class="spur">
        <div class="schiene">
          <SkeletonKarten breit={breit} n={n} />
        </div>
      </div>
    </section>
  )
}

export function SkeletonHero() {
  return (
    <div class="held held-skel" aria-busy="true">
      <div class="inhalt">
        <div class="fl-skel zeile" style={{ width: '16%' }} />
        <div class="fl-skel skel-plakat" />
        <div class="fl-skel zeile" style={{ width: '38%' }} />
        <div class="fl-skel zeile" style={{ width: '32%' }} />
      </div>
    </div>
  )
}
