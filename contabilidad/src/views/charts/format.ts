/** Compact amount for chart labels, e.g. 18450000 -> "18,5 M". The exact figure lives in the data table. */
export const formatMillions = (amount: number): string => `${(amount / 1_000_000).toFixed(1).replace('.', ',')} M`
