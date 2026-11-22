// The Up next queue: plain functions on arrays of videos, so the player's
// behaviour is easy to follow and to test (see queue.test.mjs).

// Adds a video to play next (front) or later (end), without duplicates.
export function enqueue(queue, video, where = 'end') {
  const rest = queue.filter((v) => v.id !== video.id)
  return where === 'next' ? [video, ...rest] : [...rest, video]
}

export const removeAt = (queue, i) => queue.filter((_, k) => k !== i)

// Moves the item at from to index to.
export function move(queue, from, to) {
  if (from === to || from < 0 || from >= queue.length) return queue
  const out = [...queue]
  const [v] = out.splice(from, 1)
  out.splice(Math.max(0, Math.min(to, out.length)), 0, v)
  return out
}

// Fisher-Yates with an injectable random function.
export function shuffle(list, random = Math.random) {
  const out = [...list]
  for (let i = out.length - 1; i > 0; i--) {
    const j = Math.floor(random() * (i + 1))
    ;[out[i], out[j]] = [out[j], out[i]]
  }
  return out
}

// What plays after the current video: the first queued one, if any.
export function advance(queue) {
  if (!queue.length) return { next: null, queue }
  return { next: queue[0], queue: queue.slice(1) }
}

// Play all (or shuffle) from a list starting at index i: that video now, the
// rest queued after it.
export function playFrom(list, i = 0, shuffled = false, random = Math.random) {
  if (!list.length) return { now: null, queue: [] }
  if (shuffled) {
    const order = shuffle(list, random)
    return { now: order[0], queue: order.slice(1) }
  }
  return { now: list[i], queue: list.slice(i + 1) }
}
