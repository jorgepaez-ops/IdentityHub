const unknownDevice = 'Dispositivo desconocido'

function browserOf(agent: string): string | null {
  // Order matters: Edge and Opera also announce Chrome, and Chrome also announces Safari.
  if (/Edg(e|A|iOS)?\//.test(agent)) return 'Edge'
  if (/OPR\/|Opera/.test(agent)) return 'Opera'
  if (/Firefox\/|FxiOS\//.test(agent)) return 'Firefox'
  if (/Chrome\/|CriOS\//.test(agent)) return 'Chrome'
  if (/Safari\//.test(agent)) return 'Safari'
  if (/^curl\//i.test(agent)) return 'curl'
  return null
}

function osOf(agent: string): string | null {
  // iPhone and iPad agents also say "like Mac OS X", so mobile systems are checked first.
  if (/iPhone|iPad|iPod/.test(agent)) return 'iOS'
  if (/Android/.test(agent)) return 'Android'
  if (/Windows/.test(agent)) return 'Windows'
  if (/Macintosh|Mac OS X/.test(agent)) return 'macOS'
  if (/CrOS/.test(agent)) return 'ChromeOS'
  if (/Linux|X11/.test(agent)) return 'Linux'
  return null
}

// Short, human label for a session row; the full string stays available in a title attribute.
export function describeUserAgent(agent: string): string {
  const browser = browserOf(agent)
  const system = osOf(agent)
  if (browser && system) return `${browser} en ${system}`
  return browser ?? system ?? unknownDevice
}
