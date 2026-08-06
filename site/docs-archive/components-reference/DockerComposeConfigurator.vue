<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { Check, Clipboard, Download, ServerCog } from '@lucide/vue'

type Backend = 'cpu' | 'vulkan' | 'cuda'
type Region = 'other' | 'cn'
type Preset = 'minimal' | 'basic' | 'brave' | 'custom'
type Service = 'siglip' | 'face' | 'ocr' | 'bioclip'
type CopyTarget = 'env' | 'command'

const props = withDefaults(defineProps<{ lang?: 'zh-CN' | 'en' }>(), {
  lang: 'zh-CN',
})

const messages = {
  'zh-CN': {
    eyebrow: 'Lumen Hub · Docker 向导',
    title: '生成 canonical 环境变量与启动命令',
    intro: '向导只输出薄 intent（LUMEN_* 环境变量）和启动命令；完整的运行时配置由 Lumen Hub 在容器启动时渲染并校验。',
    steps: ['计算硬件', '下载地区', '模型能力', '确认部署'],
    step: '步骤',
    hardwareTitle: 'Lumen Hub 将运行在哪种硬件上？',
    hardwareIntro: '这里选择的是计算设备，不一定是保存媒体的 NAS。',
    recommended: '不确定就选它',
    requirements: '主机要求',
    included: '命令已包含',
    toolkit: '安装 NVIDIA Container Toolkit',
    regionTitle: '模型从哪个下载源获取？',
    regionIntro: '这只影响模型下载地址，不改变界面语言和媒体位置。',
    regionOtherTitle: '全球',
    regionOtherBody: '从 Hugging Face 官方源下载',
    regionCnTitle: '中国大陆',
    regionCnBody: '使用为中国大陆准备的模型镜像',
    presetTitle: '选择需要的模型能力',
    presetIntro: '预设适合大多数设备；只有确实想改变模型组合时才选择自定义。',
    presetRecommended: '推荐',
    customTitle: '自定义能力组合',
    customIntro: '至少选择一项能力。只有存在多个受支持模型时才需要继续选择。',
    emptyCustom: '至少选择一项模型能力后才能继续。',
    fixedModel: '固定模型',
    semanticModel: '语义模型',
    speciesCatalog: '物种目录',
    reviewTitle: '确认后下载 .env 并运行',
    reviewIntro: '下载向导生成的 lumen.env，再复制并运行下面的启动命令。',
    summaryHardware: '计算后端',
    summaryRegion: '下载地区',
    summaryPreset: '能力方案',
    summaryServices: '启用能力',
    summaryResources: '资源参考',
    summaryEnvironment: 'canonical 环境变量（薄 intent）',
    custom: '自定义',
    noServices: '尚未选择',
    download: '下载 lumen.env',
    copyEnv: '复制 .env',
    copiedEnv: '.env 已复制',
    copyCommand: '复制启动命令',
    copiedCommand: '命令已复制',
    copyFailed: '浏览器未允许复制，请展开配置后手动复制。',
    preview: '查看完整 .env',
    back: '上一步',
    next: '下一步',
    networkContract: '启动命令使用 Linux host network，以保留 Lumen 的局域网 mDNS 自动发现。Hub 变为 healthy 后，Lumilio Photos 会自动连接，不需要静态地址。',
    firstStart: '第一次启动会下载所选模型。在下载、加载和预热完成前，容器保持 starting 属于正常现象。',
    envNote: 'LUMEN_* 变量是部署意图，不是完整配置：Lumen Hub 在容器启动时用它们渲染并校验完整配置，非法组合会在下载模型前失败。',
    composeDefault: '只想用默认能力时不需要本向导：发行 Compose 文件（lumen-cpu / lumen-vulkan / lumen-cuda）已经内置 basic 预设与 other 地区，直接 `docker compose -f <文件> up -d --wait` 即可。',
    customResource: (disk: string) => `模型磁盘约 ${disk} GB；内存取决于所选组合`,
    backends: {
      cpu: {
        title: '没有 GPU / 不确定',
        badge: 'CPU',
        summary: '普通 NAS、ARM 主机和低功耗服务器的稳妥选择。',
        requirement: 'Linux amd64 或 arm64；无需额外驱动',
        included: 'CPU 镜像、host network、模型持久化卷',
      },
      vulkan: {
        title: 'Intel / AMD GPU',
        badge: 'Vulkan',
        summary: '使用支持 Vulkan 1.3 的核显或独立显卡加速。',
        requirement: 'Linux amd64；主机存在 /dev/dri 且驱动支持 Vulkan 1.3',
        included: 'Vulkan 镜像、--device /dev/dri、host network',
      },
      cuda: {
        title: 'NVIDIA GPU',
        badge: 'CUDA',
        summary: '面向独立计算主机或 GPU 服务器的 NVIDIA 加速。',
        requirement: 'Linux amd64；NVIDIA 驱动与 Container Toolkit',
        included: 'CUDA 镜像、--gpus all、host network',
      },
    },
    presets: {
      minimal: {
        title: '最小',
        summary: '图像语义分析与人物识别',
        resource: 'RAM 4 GB · GPU/统一内存 2 GB · 磁盘约 2 GB',
      },
      basic: {
        title: '基础',
        summary: '图像语义分析、人物识别、OCR文字识别与BioCLIP物种识别',
        resource: 'RAM 6 GB · GPU/统一内存 3 GB · 磁盘约 6 GB',
      },
      brave: {
        title: '完整',
        summary: '更强的图像语义分析模型与完整BioCLIP物种目录',
        resource: 'RAM 8 GB · GPU/统一内存 4 GB · 磁盘约 10 GB',
      },
      custom: {
        title: '自定义',
        summary: '自行选择能力和受支持模型',
        resource: '按最终组合计算',
      },
    },
    services: {
      siglip: { title: '图像语义分析', summary: '支持自然语言搜索和相似媒体分析' },
      face: { title: '人物识别', summary: '人脸检测、特征与聚类' },
      ocr: { title: 'OCR文字识别', summary: '识别图片、截图和文档文字' },
      bioclip: { title: 'BioCLIP物种识别', summary: '植物、动物和自然观察分类' },
    },
  },
  en: {
    eyebrow: 'Lumen Hub · Docker wizard',
    title: 'Generate canonical environment variables and a start command',
    intro: 'The wizard outputs only the thin intent (LUMEN_* environment variables) and a start command. Lumen Hub renders and validates the complete runtime configuration when the container starts.',
    steps: ['Hardware', 'Download region', 'Model capabilities', 'Review'],
    step: 'Step',
    hardwareTitle: 'Which hardware will run Lumen Hub?',
    hardwareIntro: 'This is the compute node. It does not have to be the NAS that stores your media.',
    recommended: 'Choose this if unsure',
    requirements: 'Host requirements',
    included: 'Already included in the command',
    toolkit: 'Install NVIDIA Container Toolkit',
    regionTitle: 'Where should models be downloaded from?',
    regionIntro: 'This only changes model download routing, not the UI language or media location.',
    regionOtherTitle: 'Global',
    regionOtherBody: 'Download from the official Hugging Face source',
    regionCnTitle: 'Mainland China',
    regionCnBody: 'Use the model mirror prepared for mainland China',
    presetTitle: 'Choose model capabilities',
    presetIntro: 'Presets fit most devices. Choose Custom only when you need a different model combination.',
    presetRecommended: 'Recommended',
    customTitle: 'Custom capabilities',
    customIntro: 'Select at least one capability. A model selector appears only when multiple supported models exist.',
    emptyCustom: 'Select at least one model capability to continue.',
    fixedModel: 'Fixed model',
    semanticModel: 'Semantic model',
    speciesCatalog: 'Species catalog',
    reviewTitle: 'Review, download the .env, and run',
    reviewIntro: 'Download the generated lumen.env, then copy and run the start command below.',
    summaryHardware: 'Compute backend',
    summaryRegion: 'Download region',
    summaryPreset: 'Capability plan',
    summaryServices: 'Enabled capabilities',
    summaryResources: 'Resource guidance',
    summaryEnvironment: 'Canonical environment variables (thin intent)',
    custom: 'Custom',
    noServices: 'None selected',
    download: 'Download lumen.env',
    copyEnv: 'Copy .env',
    copiedEnv: '.env copied',
    copyCommand: 'Copy start command',
    copiedCommand: 'Command copied',
    copyFailed: 'Clipboard access was denied. Expand the configuration and copy it manually.',
    preview: 'View complete .env',
    back: 'Back',
    next: 'Continue',
    networkContract: 'The start command uses Linux host networking to preserve Lumen mDNS discovery. Once the Hub is healthy, Lumilio Photos connects automatically without a static address.',
    firstStart: 'The first start downloads the selected models. The container normally remains starting until download, loading, and warmup finish.',
    envNote: 'LUMEN_* variables are deployment intent, not a complete configuration: Lumen Hub renders and validates the full configuration at container startup, and invalid combinations fail before model download.',
    composeDefault: 'You do not need this wizard for the default capabilities: the published Compose files (lumen-cpu / lumen-vulkan / lumen-cuda) already embed the basic preset and the other region. Run `docker compose -f <file> up -d --wait` directly.',
    customResource: (disk: string) => `About ${disk} GB of model storage; memory depends on the selected combination`,
    backends: {
      cpu: {
        title: 'No GPU / not sure',
        badge: 'CPU',
        summary: 'A safe choice for ordinary NAS devices, ARM hosts, and low-power servers.',
        requirement: 'Linux amd64 or arm64; no additional driver',
        included: 'CPU image, host networking, persistent model volume',
      },
      vulkan: {
        title: 'Intel / AMD GPU',
        badge: 'Vulkan',
        summary: 'Accelerate with an integrated or discrete GPU that supports Vulkan 1.3.',
        requirement: 'Linux amd64; /dev/dri and a Vulkan 1.3-capable host driver',
        included: 'Vulkan image, --device /dev/dri, host networking',
      },
      cuda: {
        title: 'NVIDIA GPU',
        badge: 'CUDA',
        summary: 'NVIDIA acceleration for a dedicated compute node or GPU server.',
        requirement: 'Linux amd64; NVIDIA driver and Container Toolkit',
        included: 'CUDA image, --gpus all, host networking',
      },
    },
    presets: {
      minimal: {
        title: 'Minimal',
        summary: 'Image Semantic Analysis and Person Recognition',
        resource: '4 GB RAM · 2 GB GPU/unified memory · about 2 GB disk',
      },
      basic: {
        title: 'Basic',
        summary: 'All four Lumen Intelligence capabilities',
        resource: '6 GB RAM · 3 GB GPU/unified memory · about 6 GB disk',
      },
      brave: {
        title: 'Full',
        summary: 'Stronger Image Semantic Analysis model and full BioCLIP catalog',
        resource: '8 GB RAM · 4 GB GPU/unified memory · about 10 GB disk',
      },
      custom: {
        title: 'Custom',
        summary: 'Choose capabilities and supported models',
        resource: 'Calculated from the final combination',
      },
    },
    services: {
      siglip: { title: 'Image Semantic Analysis', summary: 'Natural-language search and similar-media analysis' },
      face: { title: 'Person Recognition', summary: 'Face detection, embeddings, and clustering' },
      ocr: { title: 'OCR Text Recognition', summary: 'Recognize text in images, screenshots, and documents' },
      bioclip: { title: 'BioCLIP Species Recognition', summary: 'Classify plants, animals, and nature observations' },
    },
  },
} as const

const backendOrder: Backend[] = ['cpu', 'vulkan', 'cuda']
const presetOrder: Preset[] = ['minimal', 'basic', 'brave', 'custom']
const serviceOrder: Service[] = ['siglip', 'face', 'ocr', 'bioclip']
const imageTag: Record<Backend, string> = {
  cpu: 'ghcr.io/edwinzhancn/lumen-hub:cpu',
  vulkan: 'ghcr.io/edwinzhancn/lumen-hub:vulkan',
  cuda: 'ghcr.io/edwinzhancn/lumen-hub:cuda',
}

const currentStep = ref(1)
const maxVisitedStep = ref(1)
const selectedBackend = ref<Backend>('cpu')
const selectedRegion = ref<Region>('other')
const selectedPreset = ref<Preset>('basic')
const customServices = reactive<Record<Service, boolean>>({
  siglip: true,
  face: true,
  ocr: true,
  bioclip: true,
})
const siglipModel = ref('siglip2-base-patch16-224')
const bioclipDataset = ref('TreeOfLife200MCore')
const copyState = ref<CopyTarget | 'error' | null>(null)

const t = computed(() => messages[props.lang])
const selectedServices = computed<Service[]>(() => {
  if (selectedPreset.value === 'minimal') return ['siglip', 'face']
  if (selectedPreset.value === 'basic' || selectedPreset.value === 'brave') {
    return [...serviceOrder]
  }
  return serviceOrder.filter((service) => customServices[service])
})
const customValid = computed(
  () => selectedPreset.value !== 'custom' || selectedServices.value.length > 0,
)

// Canonical thin intent: only the variables the Hub renderer understands.
// LUMEN_FACE_MODEL / LUMEN_OCR_MODEL are fixed by the Hub and are not accepted
// as Docker intent, so they are never emitted.
const envFileContent = computed(() => {
  const lines: string[] = [`LUMEN_REGION=${selectedRegion.value}`, `LUMEN_PRESET=${selectedPreset.value}`]

  if (selectedPreset.value === 'custom') {
    lines.push(`LUMEN_SERVICES=${selectedServices.value.join(',')}`)
    if (customServices.siglip) lines.push(`LUMEN_SIGLIP_MODEL=${siglipModel.value}`)
    if (customServices.bioclip) lines.push(`LUMEN_BIOCLIP_DATASET=${bioclipDataset.value}`)
  }
  return `${lines.join('\n')}\n`
})

const runCommand = computed(() => {
  const flags = ['-d', '--name lumen-hub', '--network host']
  if (selectedBackend.value === 'vulkan') flags.push('--device /dev/dri')
  if (selectedBackend.value === 'cuda') flags.push('--gpus all')
  flags.push('-v lumen-models:/models', '--env-file lumen.env')
  return `docker run ${flags.join(' ')} ${imageTag[selectedBackend.value]}`
})

const envFileName = 'lumen.env'
const selectedServiceLabels = computed(() =>
  selectedServices.value.map((service) => t.value.services[service].title).join('、'),
)
const selectedPresetLabel = computed(() => t.value.presets[selectedPreset.value].title)
const selectedRegionLabel = computed(() =>
  selectedRegion.value === 'cn' ? t.value.regionCnTitle : t.value.regionOtherTitle,
)
const resourceSummary = computed(() => {
  if (selectedPreset.value !== 'custom') return t.value.presets[selectedPreset.value].resource
  return t.value.customResource(customDiskEstimate.value)
})
const customDiskEstimate = computed(() => {
  let disk = 0
  if (customServices.siglip) disk += siglipModel.value.includes('so400m') ? 1.6 : 0.8
  if (customServices.face) disk += 0.5
  if (customServices.ocr) disk += 0.4
  if (customServices.bioclip) disk += bioclipDataset.value === 'TreeOfLife200M' ? 5.5 : 3.8
  return Math.max(disk, 0).toFixed(1)
})

function goToStep(step: number) {
  if (step < 1 || step > maxVisitedStep.value) return
  currentStep.value = step
  copyState.value = null
}

function nextStep() {
  if (currentStep.value === 3 && !customValid.value) return
  const next = Math.min(4, currentStep.value + 1)
  currentStep.value = next
  maxVisitedStep.value = Math.max(maxVisitedStep.value, next)
  copyState.value = null
}

function previousStep() {
  currentStep.value = Math.max(1, currentStep.value - 1)
  copyState.value = null
}

function toggleService(service: Service) {
  customServices[service] = !customServices[service]
}

async function copyText(target: CopyTarget) {
  const value = target === 'env' ? envFileContent.value : runCommand.value
  try {
    await navigator.clipboard.writeText(value)
    copyState.value = target
    window.setTimeout(() => {
      if (copyState.value === target) copyState.value = null
    }, 1800)
  } catch {
    copyState.value = 'error'
  }
}

function downloadEnv() {
  if (!customValid.value) return
  const blob = new Blob([envFileContent.value], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = envFileName
  anchor.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <section class="compose-tool" aria-labelledby="compose-tool-title">
    <header class="compose-heading">
      <span class="compose-kicker"><ServerCog :size="16" /> {{ t.eyebrow }}</span>
      <h3 id="compose-tool-title">{{ t.title }}</h3>
      <p>{{ t.intro }}</p>
    </header>

    <nav class="stepper" :aria-label="t.title">
      <button
        v-for="(label, index) in t.steps"
        :key="label"
        type="button"
        :class="{ active: currentStep === index + 1, complete: maxVisitedStep > index + 1 }"
        :disabled="maxVisitedStep < index + 1"
        @click="goToStep(index + 1)"
      >
        <span>{{ maxVisitedStep > index + 1 ? '✓' : index + 1 }}</span>
        <small>{{ label }}</small>
      </button>
    </nav>

    <div class="step-content">
      <section v-if="currentStep === 1" class="wizard-panel">
        <div class="panel-heading">
          <span>{{ t.step }} 1 / 4</span>
          <h4>{{ t.hardwareTitle }}</h4>
          <p>{{ t.hardwareIntro }}</p>
        </div>

        <div class="choice-grid backend-grid" role="radiogroup" :aria-label="t.hardwareTitle">
          <button
            v-for="backend in backendOrder"
            :key="backend"
            type="button"
            class="choice-card"
            :class="{ selected: selectedBackend === backend }"
            role="radio"
            :aria-checked="selectedBackend === backend"
            @click="selectedBackend = backend"
          >
            <span class="choice-topline">
              <span class="choice-badge">{{ t.backends[backend].badge }}</span>
              <span v-if="backend === 'cpu'" class="recommended">{{ t.recommended }}</span>
              <Check v-else-if="selectedBackend === backend" :size="17" />
            </span>
            <strong>{{ t.backends[backend].title }}</strong>
            <span>{{ t.backends[backend].summary }}</span>
          </button>
        </div>

        <div class="selection-details">
          <dl>
            <div><dt>{{ t.requirements }}</dt><dd>{{ t.backends[selectedBackend].requirement }}</dd></div>
            <div><dt>{{ t.included }}</dt><dd>{{ t.backends[selectedBackend].included }}</dd></div>
          </dl>
          <a
            v-if="selectedBackend === 'cuda'"
            href="https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html"
            target="_blank"
            rel="noreferrer"
          >{{ t.toolkit }} ↗</a>
        </div>
      </section>

      <section v-else-if="currentStep === 2" class="wizard-panel">
        <div class="panel-heading">
          <span>{{ t.step }} 2 / 4</span>
          <h4>{{ t.regionTitle }}</h4>
          <p>{{ t.regionIntro }}</p>
        </div>

        <div class="choice-grid region-grid" role="radiogroup" :aria-label="t.regionTitle">
          <button
            type="button"
            class="choice-card"
            :class="{ selected: selectedRegion === 'other' }"
            role="radio"
            :aria-checked="selectedRegion === 'other'"
            @click="selectedRegion = 'other'"
          >
            <span class="choice-topline"><span class="choice-badge">GLOBAL</span><Check v-if="selectedRegion === 'other'" :size="17" /></span>
            <strong>{{ t.regionOtherTitle }}</strong>
            <span>{{ t.regionOtherBody }}</span>
          </button>
          <button
            type="button"
            class="choice-card"
            :class="{ selected: selectedRegion === 'cn' }"
            role="radio"
            :aria-checked="selectedRegion === 'cn'"
            @click="selectedRegion = 'cn'"
          >
            <span class="choice-topline"><span class="choice-badge">CN</span><Check v-if="selectedRegion === 'cn'" :size="17" /></span>
            <strong>{{ t.regionCnTitle }}</strong>
            <span>{{ t.regionCnBody }}</span>
          </button>
        </div>
      </section>

      <section v-else-if="currentStep === 3" class="wizard-panel">
        <div class="panel-heading">
          <span>{{ t.step }} 3 / 4</span>
          <h4>{{ t.presetTitle }}</h4>
          <p>{{ t.presetIntro }}</p>
        </div>

        <div class="choice-grid preset-grid" role="radiogroup" :aria-label="t.presetTitle">
          <button
            v-for="preset in presetOrder"
            :key="preset"
            type="button"
            class="choice-card preset-card"
            :class="{ selected: selectedPreset === preset }"
            role="radio"
            :aria-checked="selectedPreset === preset"
            @click="selectedPreset = preset"
          >
            <span class="choice-topline">
              <span class="choice-badge">{{ preset }}</span>
              <span v-if="preset === 'basic'" class="recommended">{{ t.presetRecommended }}</span>
              <Check v-else-if="selectedPreset === preset" :size="17" />
            </span>
            <strong>{{ t.presets[preset].title }}</strong>
            <span>{{ t.presets[preset].summary }}</span>
            <small>{{ t.presets[preset].resource }}</small>
          </button>
        </div>

        <div v-if="selectedPreset === 'custom'" class="custom-panel">
          <div class="custom-heading">
            <strong>{{ t.customTitle }}</strong>
            <span>{{ t.customIntro }}</span>
          </div>
          <div class="service-grid">
            <article
              v-for="service in serviceOrder"
              :key="service"
              class="service-card"
              :class="{ selected: customServices[service] }"
            >
              <button
                type="button"
                class="service-toggle"
                :aria-pressed="customServices[service]"
                @click="toggleService(service)"
              >
                <span class="check-box"><Check v-if="customServices[service]" :size="15" /></span>
                <span><strong>{{ t.services[service].title }}</strong><small>{{ t.services[service].summary }}</small></span>
              </button>

              <label v-if="service === 'siglip' && customServices.siglip">
                <span>{{ t.semanticModel }}</span>
                <select v-model="siglipModel">
                  <option value="siglip2-base-patch16-224">SigLIP 2 Base · 224</option>
                  <option value="siglip2-so400m-patch14-384">SigLIP 2 SO400M · 384</option>
                </select>
              </label>
              <label v-else-if="service === 'bioclip' && customServices.bioclip">
                <span>{{ t.speciesCatalog }}</span>
                <select v-model="bioclipDataset">
                  <option value="TreeOfLife200MCore">TreeOfLife200M Core</option>
                  <option value="TreeOfLife200M">TreeOfLife200M Full</option>
                </select>
              </label>
              <p v-else-if="customServices[service]" class="fixed-model">
                {{ t.fixedModel }} · {{ service === 'face' ? 'antelopev2' : 'PP-OCRv6 Small' }}
              </p>
            </article>
          </div>
          <p v-if="!customValid" class="validation-error" role="alert">{{ t.emptyCustom }}</p>
        </div>
      </section>

      <section v-else class="wizard-panel review-panel">
        <div class="panel-heading">
          <span>{{ t.step }} 4 / 4</span>
          <h4>{{ t.reviewTitle }}</h4>
          <p>{{ t.reviewIntro }}</p>
        </div>

        <div class="review-grid">
          <dl class="review-summary">
            <div><dt>{{ t.summaryHardware }}</dt><dd>{{ t.backends[selectedBackend].badge }} · {{ t.backends[selectedBackend].title }}</dd></div>
            <div><dt>{{ t.summaryRegion }}</dt><dd>{{ selectedRegionLabel }}</dd></div>
            <div><dt>{{ t.summaryPreset }}</dt><dd>{{ selectedPresetLabel }}</dd></div>
            <div><dt>{{ t.summaryServices }}</dt><dd>{{ selectedServiceLabels || t.noServices }}</dd></div>
            <div><dt>{{ t.summaryResources }}</dt><dd>{{ resourceSummary }}</dd></div>
          </dl>

          <div class="environment-summary">
            <strong>{{ t.summaryEnvironment }}</strong>
            <code v-for="line in envFileContent.trim().split('\n')" :key="line">{{ line }}</code>
          </div>
        </div>

        <p class="network-contract">{{ t.networkContract }}</p>
        <p class="first-start">{{ t.firstStart }}</p>

        <div class="download-actions">
          <button type="button" class="download" @click="downloadEnv"><Download :size="18" /> {{ t.download }}</button>
          <button type="button" @click="copyText('env')">
            <Check v-if="copyState === 'env'" :size="17" /><Clipboard v-else :size="17" />
            {{ copyState === 'env' ? t.copiedEnv : t.copyEnv }}
          </button>
        </div>

        <div class="command-row">
          <code>{{ runCommand }}</code>
          <button type="button" @click="copyText('command')">
            <Check v-if="copyState === 'command'" :size="16" /><Clipboard v-else :size="16" />
            {{ copyState === 'command' ? t.copiedCommand : t.copyCommand }}
          </button>
        </div>
        <p v-if="copyState === 'error'" class="validation-error" role="status">{{ t.copyFailed }}</p>

        <details class="compose-preview">
          <summary>{{ t.preview }} · {{ envFileName }}</summary>
          <pre><code>{{ envFileContent }}</code></pre>
        </details>

        <p class="env-note">{{ t.envNote }}</p>
        <p class="compose-default">{{ t.composeDefault }}</p>
      </section>
    </div>

    <footer class="wizard-footer">
      <button v-if="currentStep > 1" type="button" class="back" @click="previousStep">← {{ t.back }}</button>
      <span v-else />
      <button
        v-if="currentStep < 4"
        type="button"
        class="next"
        :disabled="currentStep === 3 && !customValid"
        @click="nextStep"
      >{{ t.next }} →</button>
    </footer>
  </section>
</template>

<style scoped>
.compose-tool { margin: 28px 0; overflow: hidden; border: 1px solid var(--vp-c-divider); border-radius: 14px; background: var(--vp-c-bg); }
.compose-heading { padding: 24px 24px 20px; border-bottom: 1px solid var(--vp-c-divider); background: var(--vp-c-bg-soft); }
.compose-heading h3 { margin: 7px 0 6px; border: 0; font-size: 22px; line-height: 1.3; }
.compose-heading p, .panel-heading p { max-width: 720px; margin: 0; color: var(--vp-c-text-2); font-size: 14px; line-height: 1.55; }
.compose-kicker { display: inline-flex; align-items: center; gap: 7px; color: var(--vp-c-brand-1); font-size: 13px; font-weight: 700; }
.stepper { display: grid; grid-template-columns: repeat(4, 1fr); padding: 0 18px; border-bottom: 1px solid var(--vp-c-divider); }
.stepper button { position: relative; display: flex; align-items: center; gap: 8px; border: 0; padding: 14px 6px; color: var(--vp-c-text-3); background: transparent; cursor: pointer; }
.stepper button::after { position: absolute; right: 6px; bottom: -1px; left: 6px; height: 2px; content: ''; background: transparent; }
.stepper button.active { color: var(--vp-c-brand-1); }
.stepper button.active::after { background: var(--vp-c-brand-1); }
.stepper button.complete { color: var(--vp-c-text-2); }
.stepper button:disabled { cursor: default; opacity: .5; }
.stepper button > span { display: grid; width: 24px; height: 24px; flex: 0 0 auto; place-items: center; border: 1px solid currentColor; border-radius: 50%; font-size: 11px; font-weight: 750; }
.stepper small { overflow: hidden; font-size: 12px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.step-content { min-height: 440px; }
.wizard-panel { padding: 24px; }
.panel-heading > span { color: var(--vp-c-brand-1); font-size: 11px; font-weight: 750; letter-spacing: .05em; text-transform: uppercase; }
.panel-heading h4 { margin: 5px 0 5px; font-size: 19px; line-height: 1.35; }
.choice-grid { display: grid; gap: 12px; margin-top: 20px; }
.backend-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.region-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); max-width: 680px; }
.preset-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.choice-card { min-width: 0; border: 1px solid var(--vp-c-divider); border-radius: 10px; padding: 16px; color: var(--vp-c-text-1); background: var(--vp-c-bg); text-align: left; cursor: pointer; transition: border-color .18s ease, background-color .18s ease, transform .18s ease; }
.choice-card:hover { border-color: var(--vp-c-brand-1); transform: translateY(-1px); }
.choice-card:focus-visible, .service-toggle:focus-visible, .wizard-footer button:focus-visible, .download-actions button:focus-visible, .command-row button:focus-visible { outline: 2px solid var(--vp-c-brand-1); outline-offset: 2px; }
.choice-card.selected { border-color: var(--vp-c-brand-1); background: var(--vp-c-brand-soft); }
.choice-topline { display: flex; min-height: 22px; align-items: center; justify-content: space-between; gap: 8px; color: var(--vp-c-brand-1); }
.choice-badge { font-family: var(--vp-font-family-mono); font-size: 11px; font-weight: 750; letter-spacing: .04em; text-transform: uppercase; }
.recommended { color: var(--vp-c-text-3); font-size: 11px; font-weight: 600; }
.choice-card > strong { display: block; margin-top: 9px; font-size: 15px; }
.choice-card > span:last-child { display: block; margin-top: 6px; color: var(--vp-c-text-2); font-size: 13px; line-height: 1.45; }
.preset-card > small { display: block; margin-top: 10px; color: var(--vp-c-text-3); font-size: 11px; line-height: 1.45; }
.selection-details { margin-top: 16px; padding: 16px; border: 1px solid var(--vp-c-divider); border-radius: 10px; background: var(--vp-c-bg-soft); }
.selection-details dl { display: grid; gap: 7px; margin: 0; }
.selection-details dl div { display: grid; grid-template-columns: 108px minmax(0, 1fr); gap: 12px; font-size: 13px; line-height: 1.5; }
.selection-details dt, .review-summary dt { color: var(--vp-c-text-3); font-weight: 650; }
.selection-details dd, .review-summary dd { margin: 0; }
.selection-details a { display: inline-block; margin-top: 10px; font-size: 13px; font-weight: 650; }
.custom-panel { margin-top: 16px; padding: 17px; border: 1px solid var(--vp-c-divider); border-radius: 10px; background: var(--vp-c-bg-soft); }
.custom-heading { display: flex; flex-direction: column; gap: 3px; }
.custom-heading strong { font-size: 14px; }
.custom-heading span { color: var(--vp-c-text-2); font-size: 12px; line-height: 1.45; }
.service-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; margin-top: 14px; }
.service-card { overflow: hidden; border: 1px solid var(--vp-c-divider); border-radius: 9px; background: var(--vp-c-bg); }
.service-card.selected { border-color: var(--vp-c-brand-1); }
.service-toggle { display: grid; width: 100%; grid-template-columns: 20px minmax(0, 1fr); gap: 9px; border: 0; padding: 13px; color: var(--vp-c-text-1); background: transparent; text-align: left; cursor: pointer; }
.check-box { display: grid; width: 18px; height: 18px; place-items: center; border: 1px solid var(--vp-c-divider); border-radius: 5px; color: white; }
.service-card.selected .check-box { border-color: var(--vp-c-brand-1); background: var(--vp-c-brand-1); }
.service-toggle strong, .service-toggle small { display: block; }
.service-toggle strong { font-size: 13px; }
.service-toggle small { margin-top: 3px; color: var(--vp-c-text-3); font-size: 11px; line-height: 1.4; }
.service-card label, .fixed-model { display: flex; align-items: center; justify-content: space-between; gap: 9px; margin: 0; padding: 0 13px 13px 42px; color: var(--vp-c-text-3); font-size: 11px; }
.service-card select { min-width: 0; max-width: 190px; border: 1px solid var(--vp-c-divider); border-radius: 6px; padding: 5px 7px; color: var(--vp-c-text-1); background: var(--vp-c-bg); font: inherit; }
.review-grid { display: grid; grid-template-columns: minmax(0, 1.15fr) minmax(230px, .85fr); gap: 14px; margin-top: 20px; }
.review-summary, .environment-summary { margin: 0; padding: 17px; border: 1px solid var(--vp-c-divider); border-radius: 10px; background: var(--vp-c-bg-soft); }
.review-summary { display: grid; gap: 9px; }
.review-summary div { display: grid; grid-template-columns: 105px minmax(0, 1fr); gap: 12px; font-size: 12px; line-height: 1.5; }
.environment-summary { display: flex; flex-direction: column; gap: 7px; min-width: 0; }
.environment-summary strong { margin-bottom: 3px; font-size: 12px; }
.environment-summary code { overflow-wrap: anywhere; color: var(--vp-c-text-2); font-size: 10px; }
.network-contract, .first-start, .env-note, .compose-default { margin: 14px 0 0; padding: 12px 14px; color: var(--vp-c-text-2); background: var(--vp-c-bg-soft); font-size: 12px; line-height: 1.5; }
.network-contract { border-left: 3px solid var(--vp-c-brand-1); }
.first-start { margin-top: 8px; }
.env-note { margin-top: 8px; border-left: 3px solid var(--vp-c-brand-1); }
.compose-default { margin-top: 8px; }
.download-actions { display: flex; gap: 9px; margin-top: 16px; }
.download-actions button, .command-row button, .wizard-footer button { display: inline-flex; align-items: center; justify-content: center; gap: 7px; border: 1px solid var(--vp-c-divider); border-radius: 8px; padding: 9px 13px; color: var(--vp-c-text-1); background: var(--vp-c-bg); font-size: 13px; font-weight: 700; cursor: pointer; }
.download-actions .download, .wizard-footer .next { border-color: var(--vp-c-brand-1); color: var(--vp-c-bg); background: var(--vp-c-brand-1); }
.command-row { display: flex; align-items: center; justify-content: space-between; gap: 14px; margin-top: 10px; padding: 10px 10px 10px 14px; border: 1px solid var(--vp-c-divider); border-radius: 9px; background: var(--vp-code-block-bg); }
.command-row code { overflow-wrap: anywhere; color: var(--vp-c-text-1); font-size: 11px; }
.command-row button { flex: 0 0 auto; padding: 7px 10px; }
.validation-error { margin: 10px 0 0; color: var(--vp-c-danger-1); font-size: 12px; }
.compose-preview { margin-top: 12px; border: 1px solid var(--vp-c-divider); border-radius: 9px; overflow: hidden; }
.compose-preview summary { padding: 11px 13px; color: var(--vp-c-text-2); background: var(--vp-c-bg-soft); font-size: 12px; font-weight: 650; cursor: pointer; }
.compose-preview pre { margin: 0; padding: 16px; overflow: auto; border-radius: 0; background: var(--vp-code-block-bg); }
.compose-preview code { font-size: 11px; line-height: 1.5; }
.wizard-footer { display: flex; min-height: 63px; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 24px; border-top: 1px solid var(--vp-c-divider); background: var(--vp-c-bg-soft); }
.wizard-footer button:disabled { cursor: not-allowed; opacity: .45; }

@media (max-width: 760px) {
  .backend-grid, .preset-grid, .service-grid, .review-grid { grid-template-columns: 1fr; }
  .stepper { padding: 0 8px; }
  .stepper button { justify-content: center; }
  .stepper small { display: none; }
  .selection-details dl div, .review-summary div { grid-template-columns: 1fr; gap: 2px; }
  .region-grid { grid-template-columns: 1fr; }
}

@media (max-width: 480px) {
  .compose-heading, .wizard-panel { padding-right: 16px; padding-left: 16px; }
  .wizard-footer { padding-right: 16px; padding-left: 16px; }
  .download-actions, .command-row { align-items: stretch; flex-direction: column; }
  .service-card label, .fixed-model { align-items: stretch; flex-direction: column; padding-left: 42px; }
  .service-card select { max-width: none; }
}

@media (prefers-reduced-motion: reduce) {
  .choice-card { transition: none; }
}
</style>
