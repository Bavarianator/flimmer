// Hisense-Fernseher mit VIDAA OS (z. B. 43 Zoll A6/A7/E7). Modelle mit Google TV nutzen die Android-App (Leanback).
// - Erkennung: Der VIDAA-Browser nennt „VIDAA“ oder „Hisense“ im User-Agent. Hisense-Handys heißen auch so,
//   deshalb zählt „Hisense“ nur ohne „Mobile“.
// - Fernbedienung: Zurück 8/27, Play 415, Pause 19, Stopp 413, Spulen 412/417. Das fangen router.ts und der Player schon ab.
// - Formate: keine festen Regeln wie bei webOS/Tizen. VIDAA spielt nicht zuverlässig MKV/TS und kaum HDR im Browser,
//   also gelten canPlayType und die Messung aus „Gerät neu testen“.
// - Größe: ältere Modelle rechnen mit 1280 × 720. Das fängt der TV-Viewport (width=1920) in device.ts ab.
const ua = navigator.userAgent
export const isHisense = /VIDAA/i.test(ua) || (/Hisense/i.test(ua) && !/Mobile/i.test(ua))
