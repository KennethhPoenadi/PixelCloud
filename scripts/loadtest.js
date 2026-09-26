// k6 load test: a mix of browsing (reads) and rendering (job submissions).
// Usually run through `make loadtest`, which uses the grafana/k6 image on the
// compose network. Env: BASE_URL, PASSWORD, VUS, DURATION, JOB_RATIO.
// Accounts loadtest-1..N@pixelcloud.local come from scripts/prepare-loadtest-user.sh.
import http from 'k6/http'
import { check, sleep } from 'k6'

const BASE = `${__ENV.BASE_URL || 'http://localhost:8080'}/api/v1`
const PASSWORD = __ENV.PASSWORD || 'loadtest-password'
const VUS = Number(__ENV.VUS || 10)
const JOB_RATIO = Number(__ENV.JOB_RATIO || 0.2)
const SAMPLE = open(__ENV.SAMPLE || '../frontend/e2e/fixtures/sample.jpg', 'b')
const PRESETS = ['grayscale', 'sepia', 'vintage', 'warm', 'cool', 'vivid', 'noir', 'fade', 'invert']

export const options = {
  scenarios: {
    browse_and_render: {
      executor: 'constant-vus',
      vus: VUS,
      duration: __ENV.DURATION || '3m',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    'http_req_duration{kind:read}': ['p(95)<500'],
  },
}

const json = { headers: { 'Content-Type': 'application/json' } }

// One account per VU so each stays under the per-user API rate limit.
export function setup() {
  const sessions = []
  for (let i = 1; i <= VUS; i++) {
    const email = `loadtest-${i}@pixelcloud.local`
    const login = http.post(`${BASE}/auth/login`, JSON.stringify({ email, password: PASSWORD }), json)
    check(login, { 'login ok': (r) => r.status === 200 })
    const token = login.json('token')
    const auth = { headers: { Authorization: `Bearer ${token}` } }
    const up = http.post(`${BASE}/images`, { file: http.file(SAMPLE, 'loadtest.jpg', 'image/jpeg') }, auth)
    check(up, { 'upload ok': (r) => r.status === 201 })
    sessions.push({ token, imageId: up.json('id') })
  }
  return { sessions }
}

export default function ({ sessions }) {
  const data = sessions[(__VU - 1) % sessions.length]
  const params = { headers: { Authorization: `Bearer ${data.token}` }, tags: { kind: 'read' } }
  const responses = http.batch([
    ['GET', `${BASE}/me`, null, params],
    ['GET', `${BASE}/images?limit=24`, null, params],
    ['GET', `${BASE}/presets`, null, params],
  ])
  check(responses[1], { 'list images 200': (r) => r.status === 200 })

  if (Math.random() < JOB_RATIO) {
    const body = {
      image_id: data.imageId,
      pipeline: {
        version: 1,
        operations: [
          { op: 'preset', name: PRESETS[Math.floor(Math.random() * PRESETS.length)] },
          { op: 'brightness', value: 1 + Math.random() * 0.3 },
          { op: 'resize', max_width: 640 },
        ],
      },
      output_format: 'jpeg',
    }
    const res = http.post(`${BASE}/jobs`, JSON.stringify(body), {
      headers: { ...params.headers, 'Content-Type': 'application/json' },
      tags: { kind: 'write' },
    })
    check(res, { 'job accepted': (r) => r.status === 202 })
  }
  sleep(0.5 + Math.random())
}
