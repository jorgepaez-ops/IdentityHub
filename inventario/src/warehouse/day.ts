const twoDigits = (value: number): string => String(value).padStart(2, '0')

/** The calendar day (YYYY-MM-DD) in the device's own time zone; `toISOString` would give the UTC day. */
export function localDay(moment: Date): string {
  return `${moment.getFullYear()}-${twoDigits(moment.getMonth() + 1)}-${twoDigits(moment.getDate())}`
}
