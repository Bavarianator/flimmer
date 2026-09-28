// hls.js liefert für den light-Build keine eigenen Typen; API ist identisch.
declare module 'hls.js/light' {
  export * from 'hls.js'
  export { default } from 'hls.js'
}
