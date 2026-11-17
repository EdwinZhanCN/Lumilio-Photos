declare module 'virtual:lumilio-atlas' {
  const data: import('./model').AtlasData | null
  export default data
}
declare module 'virtual:lumilio-atlas-symbols' {
  const symbols: import('./model').AtlasSymbol[]
  export default symbols
}
