// Run with: npm test (plain Node, no test framework needed).
import assert from 'node:assert/strict'
import { advance, enqueue, move, playFrom, removeAt, shuffle } from './queue.js'

const v = (id) => ({ id })
const ids = (q) => q.map((x) => x.id).join('')
let n = 0
const test = (name, fn) => { fn(); n++; console.log('ok', name) }

test('enqueue next and end, no duplicates', () => {
  let q = enqueue([], v('a'))
  q = enqueue(q, v('b'))
  q = enqueue(q, v('c'), 'next')
  assert.equal(ids(q), 'cab')
  assert.equal(ids(enqueue(q, v('a'), 'next')), 'acb')
  assert.equal(ids(enqueue(q, v('c'))), 'abc')
})

test('remove and move', () => {
  const q = [v('a'), v('b'), v('c'), v('d')]
  assert.equal(ids(removeAt(q, 1)), 'acd')
  assert.equal(ids(move(q, 0, 2)), 'bcad')
  assert.equal(ids(move(q, 3, 0)), 'dabc')
  assert.equal(ids(move(q, 1, 99)), 'acdb')
  assert.equal(move(q, 5, 0), q)
})

test('advance takes the front', () => {
  const { next, queue } = advance([v('a'), v('b')])
  assert.equal(next.id, 'a')
  assert.equal(ids(queue), 'b')
  assert.equal(advance([]).next, null)
})

test('play all and shuffle', () => {
  const list = [v('a'), v('b'), v('c'), v('d')]
  const p = playFrom(list, 1)
  assert.equal(p.now.id, 'b')
  assert.equal(ids(p.queue), 'cd')
  const seq = [0.9, 0.1, 0.5]
  let k = 0
  const s = playFrom(list, 0, true, () => seq[k++ % seq.length])
  assert.equal(ids([s.now, ...s.queue]).split('').sort().join(''), 'abcd') // same videos, new order
  assert.equal(shuffle([]).length, 0)
})

console.log(`${n} tests passed`)
