<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Check, Clipboard, Download, RefreshCw, ServerCog } from '@lucide/vue'

type NetworkMode = 'disabled' | 'host' | 'broker' | 'static'
type TranscodeMode = 'cpu' | 'vaapi' | 'nvenc'

const releaseTag = ref('')
const releaseState = ref<'loading' | 'ready' | 'error'>('loading')
const storagePath = ref('/srv/lumilio')
const httpPort = ref(6657)
const httpsPort = ref(6658)
const exposeAPI = ref(false)
const apiPort = ref(6680)
const networkMode = ref<NetworkMode>('disabled')
const brokerURL = ref('http://host.docker.internal:5866')
const staticNodes = ref('')
const transcodeMode = ref<TranscodeMode>('cpu')
const copied = ref(false)

const imageTag = computed(() => releaseTag.value.trim().replace(/^v/, ''))
const pathValid = computed(() => storagePath.value.trim().startsWith('/'))
const portValid = (port: number) => Number.isInteger(port) && port > 0 && port <= 65535
const canDownload = computed(
  () => imageTag.value !== '' && pathValid.value && portValid(httpPort.value) && portValid(httpsPort.value) && (!exposeAPI.value || portValid(apiPort.value)),
)

async function loadLatestRelease() {
  releaseState.value = 'loading'
  try {
    const response = await fetch('https://api.github.com/repos/EdwinZhanCN/Lumilio-Photos/releases?per_page=10')
    if (!response.ok) throw new Error(`GitHub HTTP ${response.status}`)
    const releases = (await response.json()) as Array<{ draft?: boolean; tag_name?: string }>
    const latest = releases.find((release) => !release.draft && release.tag_name)
    if (!latest?.tag_name) throw new Error('No published release')
    releaseTag.value = latest.tag_name
    releaseState.value = 'ready'
  } catch {
    releaseState.value = 'error'
  }
}

function yamlQuote(value: string) {
  return JSON.stringify(value)
}

const composeYAML = computed(() => {
  const tag = imageTag.value || '<release-version>'
  const hostNetwork = networkMode.value === 'host'
  const serverPorts = exposeAPI.value && !hostNetwork ? `\n    ports:\n      - "${apiPort.value}:6680"` : ''
  const serverNetwork = hostNetwork ? '\n    network_mode: host' : ''
  const dbPorts = hostNetwork ? '\n    ports:\n      - "127.0.0.1:5433:5432"' : ''
  const dbHost = hostNetwork ? '127.0.0.1' : 'db'
  const dbPort = hostNetwork ? '5433' : '5432'
  const mdns = networkMode.value === 'host' ? 'true' : 'false'
  const broker = networkMode.value === 'broker' ? brokerURL.value.trim() : ''
  const nodes = networkMode.value === 'static' ? staticNodes.value.trim() : ''
  const apiUpstream = hostNetwork ? 'http://host.docker.internal:6680' : 'http://server:6680'
  const extraHosts = networkMode.value === 'broker'
    ? '\n    extra_hosts:\n      - "host.docker.internal:host-gateway"'
    : ''
  const transcodeAccel = transcodeMode.value === 'vaapi' ? 'vaapi' : transcodeMode.value === 'nvenc' ? 'nvenc' : 'none'
  const devices = transcodeMode.value === 'vaapi' ? '\n    devices:\n      - /dev/dri:/dev/dri' : ''
  const nvidia = transcodeMode.value === 'nvenc'
    ? '\n    deploy:\n      resources:\n        reservations:\n          devices:\n            - driver: nvidia\n              count: 1\n              capabilities: [gpu]'
    : ''

  return `name: lumilio-photos

services:
  db:
    image: ghcr.io/edwinzhancn/lumilio-db:${tag}
    restart: unless-stopped
    environment:
      POSTGRES_DB: lumiliophotos
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - db_data:/var/lib/postgresql/data${dbPorts}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d lumiliophotos"]
      interval: 10s
      timeout: 5s
      retries: 5

  server:
    image: ghcr.io/edwinzhancn/lumilio-server:${tag}
    restart: unless-stopped
    depends_on:
      db:
        condition: service_healthy${serverNetwork}
    environment:
      SERVER_PORT: 6680
      SERVER_ENV: production
      DB_HOST: ${dbHost}
      DB_PORT: ${dbPort}
      DB_USER: postgres
      DB_PASSWORD: postgres
      DB_NAME: lumiliophotos
      DB_SSL: disable
      STORAGE_PATH: /data/storage
      LUMILIO_DB_PASSWORD_FILE: /data/storage/.secrets/db_password
      LUMILIO_SECRET_KEY: /data/storage/.secrets/lumilio_secret_key
      TRANSCODE_HW_ACCEL: ${transcodeAccel}
      LUMEN_DISCOVERY_MDNS_ENABLED: "${mdns}"
      LUMEN_DISCOVERY_HUB_URL: ${yamlQuote(broker)}
      LUMEN_DISCOVERY_STATIC_NODES: ${yamlQuote(nodes)}
    volumes:
      - ${storagePath.value.trim() || '/srv/lumilio'}:/data/storage${serverPorts}${extraHosts}${devices}${nvidia}
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://localhost:6680/api/v1/health >/dev/null"]
      interval: 10s
      timeout: 5s
      retries: 5
      start_period: 20s

  web:
    image: ghcr.io/edwinzhancn/lumilio-web:${tag}
    restart: unless-stopped
    depends_on:
      server:
        condition: service_healthy
    environment:
      LUMILIO_SITE_ADDRESS: ":80"
      LUMILIO_API_UPSTREAM: ${apiUpstream}
    ports:
      - "${httpPort.value}:80"
      - "${httpsPort.value}:443"
      - "${httpsPort.value}:443/udp"${hostNetwork ? '\n    extra_hosts:\n      - "host.docker.internal:host-gateway"' : ''}

volumes:
  db_data:
`
})

async function copyYAML() {
  if (!canDownload.value) return
  await navigator.clipboard.writeText(composeYAML.value)
  copied.value = true
  window.setTimeout(() => (copied.value = false), 1600)
}

function downloadYAML() {
  if (!canDownload.value) return
  const blob = new Blob([composeYAML.value], { type: 'application/yaml;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = 'docker-compose.yml'
  anchor.click()
  URL.revokeObjectURL(url)
}

onMounted(loadLatestRelease)
</script>

<template>
  <section class="compose-tool" aria-labelledby="compose-tool-title">
    <div class="compose-heading">
      <div>
        <span class="compose-kicker"><ServerCog :size="16" /> Docker Compose</span>
        <h3 id="compose-tool-title">生成适合你的配置</h3>
        <p>所有内容只在当前浏览器中生成，不会上传路径或配置。</p>
      </div>
      <div class="release-status" :data-state="releaseState">
        <RefreshCw v-if="releaseState === 'loading'" :size="15" class="spin" />
        <Check v-else-if="releaseState === 'ready'" :size="15" />
        <span>{{ releaseState === 'loading' ? '读取最新 Release' : releaseState === 'ready' ? `Release ${releaseTag}` : '请手动填写版本' }}</span>
      </div>
    </div>

    <div class="compose-workspace">
      <form class="compose-form" @submit.prevent>
        <label>
          <span>镜像版本</span>
          <input v-model="releaseTag" placeholder="例如 v0.1.0-beta.1" />
          <small>Beta 使用 Release 对应的精确版本，不使用 edge。</small>
        </label>

        <label>
          <span>宿主机 storage root</span>
          <input v-model="storagePath" placeholder="/srv/lumilio" />
          <small v-if="!pathValid" class="field-error">请输入 Linux 宿主机绝对路径。</small>
        </label>

        <div class="field-pair">
          <label><span>HTTP 端口</span><input v-model.number="httpPort" type="number" min="1" max="65535" /></label>
          <label><span>HTTPS 端口</span><input v-model.number="httpsPort" type="number" min="1" max="65535" /></label>
        </div>

        <label>
          <span>Lumen 发现方式</span>
          <select v-model="networkMode">
            <option value="disabled">暂不启用</option>
            <option value="host">Linux Host network + mDNS</option>
            <option value="broker">Lumen Host Broker</option>
            <option value="static">静态节点地址</option>
          </select>
        </label>

        <label v-if="networkMode === 'broker'">
          <span>Host Broker URL</span>
          <input v-model="brokerURL" placeholder="http://host.docker.internal:5866" />
        </label>

        <label v-if="networkMode === 'static'">
          <span>节点地址</span>
          <input v-model="staticNodes" placeholder="192.168.1.10:50051,192.168.1.11:50051" />
        </label>

        <label class="check-row">
          <input v-model="exposeAPI" type="checkbox" />
          <span>向宿主机公开 API 端口</span>
        </label>
        <label v-if="exposeAPI"><span>API 端口</span><input v-model.number="apiPort" type="number" min="1" max="65535" /></label>

        <label>
          <span>视频转码方式</span>
          <select v-model="transcodeMode">
            <option value="cpu">CPU（兼容性优先）</option>
            <option value="vaapi">Intel / AMD GPU（Linux VAAPI）</option>
            <option value="nvenc">NVIDIA GPU（NVENC）</option>
          </select>
          <small v-if="transcodeMode === 'vaapi'">需要宿主机提供 <code>/dev/dri/renderD128</code> 并允许容器访问。</small>
          <small v-else-if="transcodeMode === 'nvenc'">需要 NVIDIA Container Toolkit，且当前镜像中的 FFmpeg 必须包含 <code>h264_nvenc</code>。</small>
          <small v-else>使用 libx264，不需要向容器开放 GPU 设备。</small>
        </label>
      </form>

      <div class="compose-output">
        <div class="output-actions">
          <span>docker-compose.yml</span>
          <div>
            <button type="button" :disabled="!canDownload" @click="copyYAML"><Check v-if="copied" :size="16" /><Clipboard v-else :size="16" />{{ copied ? '已复制' : '复制' }}</button>
            <button type="button" :disabled="!canDownload" class="download" @click="downloadYAML"><Download :size="16" />下载</button>
          </div>
        </div>
        <pre><code>{{ composeYAML }}</code></pre>
      </div>
    </div>
  </section>
</template>

<style scoped>
.compose-tool { margin: 28px 0; border: 1px solid var(--vp-c-divider); border-radius: 12px; overflow: hidden; background: var(--vp-c-bg); }
.compose-heading { display: flex; justify-content: space-between; gap: 24px; padding: 24px; border-bottom: 1px solid var(--vp-c-divider); }
.compose-heading h3 { margin: 6px 0 4px; font-size: 22px; line-height: 1.25; }
.compose-heading p { margin: 0; color: var(--vp-c-text-2); font-size: 14px; }
.compose-kicker, .release-status { display: inline-flex; align-items: center; gap: 7px; color: var(--vp-c-brand-1); font-size: 13px; font-weight: 650; }
.release-status { align-self: flex-start; color: var(--vp-c-text-2); white-space: nowrap; }
.release-status[data-state='ready'] { color: var(--vp-c-green-1); }
.release-status[data-state='error'] { color: var(--vp-c-warning-1); }
.compose-workspace { display: grid; grid-template-columns: minmax(260px, 0.78fr) minmax(360px, 1.22fr); min-height: 620px; }
.compose-form { display: flex; flex-direction: column; gap: 17px; padding: 24px; border-right: 1px solid var(--vp-c-divider); }
.compose-form label { display: flex; flex-direction: column; gap: 7px; color: var(--vp-c-text-1); font-size: 13px; font-weight: 600; }
.compose-form input, .compose-form select { width: 100%; box-sizing: border-box; border: 1px solid var(--vp-c-divider); border-radius: 7px; padding: 9px 11px; color: var(--vp-c-text-1); background: var(--vp-c-bg-soft); font: inherit; font-weight: 450; }
.compose-form input:focus, .compose-form select:focus { outline: 2px solid var(--vp-c-brand-soft); border-color: var(--vp-c-brand-1); }
.compose-form small { color: var(--vp-c-text-3); font-weight: 450; line-height: 1.45; }
.compose-form .field-error { color: var(--vp-c-danger-1); }
.field-pair { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
.compose-form .check-row { flex-direction: row; align-items: center; gap: 10px; font-weight: 500; }
.check-row input { width: 16px; height: 16px; accent-color: var(--vp-c-brand-1); }
.compose-output { min-width: 0; background: var(--vp-code-block-bg); }
.output-actions { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px; border-bottom: 1px solid var(--vp-c-divider); color: var(--vp-c-text-2); font-size: 13px; }
.output-actions > div { display: flex; gap: 8px; }
.output-actions button { display: inline-flex; align-items: center; gap: 6px; border: 1px solid var(--vp-c-divider); border-radius: 6px; padding: 6px 10px; color: var(--vp-c-text-1); background: var(--vp-c-bg); font-weight: 600; cursor: pointer; }
.output-actions button.download { border-color: var(--vp-c-brand-1); color: var(--vp-c-brand-1); }
.output-actions button:disabled { cursor: not-allowed; opacity: .45; }
.compose-output pre { height: 568px; margin: 0; padding: 18px; overflow: auto; border-radius: 0; background: transparent; }
.compose-output code { font-size: 12px; line-height: 1.55; }
.spin { animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 900px) { .compose-heading { flex-direction: column; } .compose-workspace { grid-template-columns: 1fr; } .compose-form { border-right: 0; border-bottom: 1px solid var(--vp-c-divider); } .compose-output pre { height: 480px; } }
@media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
