/** Pairs each message with a stable React key; repeated messages get an occurrence suffix so keys stay unique without using the array index. */
export function keyedMessages(messages: readonly string[]): { key: string; message: string }[] {
  const seen = new Map<string, number>()
  return messages.map((message) => {
    const occurrence = (seen.get(message) ?? 0) + 1
    seen.set(message, occurrence)
    return { key: `${message}#${occurrence}`, message }
  })
}
