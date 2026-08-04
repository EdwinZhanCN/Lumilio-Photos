<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Check, Clipboard, Cpu, Download, FileCode2, Play, ServerCog } from "@lucide/vue";

type Region = "other" | "cn";
type SemanticModel = "siglip2-base-patch16-224" | "siglip2-so400m-patch14-384";
type BioClipDataset = "TreeOfLife200MCore" | "TreeOfLife200M";

interface BackendOption {
  id: string;
  label: string;
  profile: string;
  note: string;
}

interface SystemOption {
  id: string;
  label: string;
  backends: BackendOption[];
}

const systems: SystemOption[] = [
  {
    id: "darwin-arm64",
    label: "macOS · Apple Silicon",
    backends: [
      {
        id: "metal",
        label: "Metal（推荐）",
        profile: "darwin-arm64-metal",
        note: "使用 Apple GPU 与统一内存。",
      },
      {
        id: "cpu",
        label: "CPU",
        profile: "darwin-arm64-cpu",
        note: "兼容性优先，不使用 GPU。",
      },
    ],
  },
  {
    id: "windows-x64",
    label: "Windows · x64",
    backends: [
      {
        id: "gpu",
        label: "GPU（推荐）",
        profile: "windows-x64-gpu",
        note: "通过 wgpu 自动使用合适的图形后端。",
      },
      {
        id: "cpu",
        label: "CPU",
        profile: "windows-x64-cpu",
        note: "无需 GPU 驱动支持。",
      },
    ],
  },
  {
    id: "linux-x64",
    label: "Linux · x64",
    backends: [
      {
        id: "cuda",
        label: "CUDA · NVIDIA",
        profile: "linux-x64-cuda",
        note: "适合已经安装 NVIDIA 驱动的主机。",
      },
      {
        id: "rocm",
        label: "ROCm · AMD",
        profile: "linux-x64-rocm",
        note: "适合具有可用 ROCm 运行时的 AMD GPU。",
      },
      {
        id: "gpu",
        label: "GPU · Vulkan / wgpu",
        profile: "linux-x64-gpu",
        note: "适合 Intel 核显或支持 Vulkan 的 AMD GPU。",
      },
      {
        id: "cpu",
        label: "CPU",
        profile: "linux-x64-cpu",
        note: "兼容性最高，不需要 GPU。",
      },
    ],
  },
  {
    id: "linux-arm64",
    label: "Linux · ARM64",
    backends: [
      {
        id: "jetson",
        label: "CUDA · NVIDIA Jetson",
        profile: "linux-arm64-jetson",
        note: "仅用于 Jetson / L4T 设备。",
      },
      {
        id: "gpu",
        label: "GPU · Vulkan / wgpu",
        profile: "linux-arm64-gpu",
        note: "用于具有可用图形驱动的 ARM64 主机。",
      },
      {
        id: "cpu",
        label: "CPU（通用）",
        profile: "linux-arm64-cpu",
        note: "适合普通 ARM64 单板机和服务器。",
      },
    ],
  },
];

const region = ref<Region>("other");
const cacheDir = ref("");
const port = ref(50051);
const serviceName = ref("lumen-hub-1");

const semanticEnabled = ref(true);
const semanticModel = ref<SemanticModel>("siglip2-base-patch16-224");
const faceEnabled = ref(true);
const ocrEnabled = ref(true);
const bioClipEnabled = ref(true);
const bioClipDataset = ref<BioClipDataset>("TreeOfLife200MCore");

const selectedSystemId = ref("linux-x64");
const selectedBackendId = ref("cuda");
const configCopied = ref(false);
const commandCopied = ref(false);

const absolutePathValid = computed(() => {
  const value = cacheDir.value.trim();
  const unixPath = value.startsWith("/") && value !== "/";
  const windowsPath = /^[A-Za-z]:[\\/].+/.test(value) && !/^[A-Za-z]:[\\/]?$/.test(value);
  const uncPath = /^\\\\[^\\]+\\[^\\]+/.test(value);
  return unixPath || windowsPath || uncPath;
});

const portValid = computed(
  () => Number.isInteger(port.value) && port.value >= 1024 && port.value <= 65535,
);
const serviceNameValid = computed(() => /^[a-z][a-z0-9-]*$/.test(serviceName.value.trim()));
const selectedServiceCount = computed(
  () =>
    Number(semanticEnabled.value) +
    Number(faceEnabled.value) +
    Number(ocrEnabled.value) +
    Number(bioClipEnabled.value),
);
const configValid = computed(
  () =>
    absolutePathValid.value &&
    portValid.value &&
    serviceNameValid.value &&
    selectedServiceCount.value > 0,
);

const selectedSystem = computed(
  () => systems.find((system) => system.id === selectedSystemId.value) ?? systems[0],
);
const selectedBackend = computed(
  () =>
    selectedSystem.value.backends.find((backend) => backend.id === selectedBackendId.value) ??
    selectedSystem.value.backends[0],
);

watch(selectedSystemId, () => {
  selectedBackendId.value = selectedSystem.value.backends[0].id;
});

function yamlSingleQuoted(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
}

const selectedServiceNames = computed(() => {
  const services: string[] = [];
  if (semanticEnabled.value) services.push("siglip");
  if (faceEnabled.value) services.push("face");
  if (ocrEnabled.value) services.push("ocr");
  if (bioClipEnabled.value) services.push("bioclip");
  return services;
});

const servicesYAML = computed(() => {
  const sections: string[] = [];

  if (semanticEnabled.value) {
    sections.push(`  siglip:
    enabled: true
    package: siglip
    models:
      default:
        model: ${semanticModel.value}
        runtime: burn
        precision: fp16q8`);
  }

  if (faceEnabled.value) {
    sections.push(`  face:
    enabled: true
    package: insightface
    models:
      default:
        model: antelopev2
        runtime: burn
        precision: fp16q8`);
  }

  if (ocrEnabled.value) {
    sections.push(`  ocr:
    enabled: true
    package: ppocr
    models:
      default:
        model: pp-ocrv6-small
        runtime: burn
        precision: fp16q8`);
  }

  if (bioClipEnabled.value) {
    sections.push(`  bioclip:
    enabled: true
    package: clip
    models:
      default:
        model: bioclip-2
        runtime: burn
        precision: fp16q8
        dataset: ${bioClipDataset.value}`);
  }

  return sections.join("\n\n");
});

const configYAML = computed(() => {
  const serviceList =
    selectedServiceNames.value.length > 0
      ? `\n${selectedServiceNames.value.map((service) => `    - ${service}`).join("\n")}`
      : " []";

  const serviceDefinitions =
    servicesYAML.value === "" ? "services: {}" : `services:\n${servicesYAML.value}`;

  return `metadata:
  version: "0.1.0"
  region: ${region.value}
  cache_dir: ${yamlSingleQuoted(cacheDir.value.trim())}

deployment:
  mode: hub
  services:${serviceList}

server:
  host: "0.0.0.0"
  port: ${port.value}
  mdns:
    enabled: true
    service_name: ${serviceName.value.trim()}
  batching:
    enabled: false
    max_batch_size: 8
    queue_latency_ms: 2

${serviceDefinitions}
`;
});

const commandText = computed(
  () => `lumen-cli validate --config ./lumen-config.yaml
lumen-cli start --config ./lumen-config.yaml --profile ${selectedBackend.value.profile}`,
);

async function copyConfig() {
  if (!configValid.value) return;
  await navigator.clipboard.writeText(configYAML.value);
  configCopied.value = true;
  window.setTimeout(() => (configCopied.value = false), 1600);
}

function downloadConfig() {
  if (!configValid.value) return;
  const blob = new Blob([configYAML.value], {
    type: "application/yaml;charset=utf-8",
  });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = "lumen-config.yaml";
  anchor.click();
  URL.revokeObjectURL(url);
}

async function copyCommand() {
  await navigator.clipboard.writeText(commandText.value);
  commandCopied.value = true;
  window.setTimeout(() => (commandCopied.value = false), 1600);
}
</script>

<template>
  <section class="lumen-builder" aria-labelledby="lumen-builder-title">
    <header class="builder-heading">
      <div class="builder-title">
        <span class="step-number">1</span>
        <div>
          <span class="builder-kicker">
            <ServerCog :size="16" aria-hidden="true" />
            Lumen Hub
          </span>
          <h3 id="lumen-builder-title">生成自定义配置</h3>
          <p>配置只在当前浏览器中生成，不会上传模型路径或设备信息。</p>
        </div>
      </div>
      <span class="fixed-summary">局域网 · mDNS · Burn · fp16q8</span>
    </header>

    <div class="config-workspace">
      <form class="config-form" @submit.prevent>
        <fieldset>
          <legend>节点设置</legend>

          <label class="field">
            <span>下载地区</span>
            <select v-model="region">
              <option value="other">其他地区 · Hugging Face</option>
              <option value="cn">中国大陆 · hf-mirror.com</option>
            </select>
          </label>

          <label class="field">
            <span>模型缓存绝对路径</span>
            <input v-model="cacheDir" autocomplete="off" placeholder="/home/lumen/.lumen/models" />
            <small v-if="!absolutePathValid" class="field-error">
              请输入具体的绝对路径，不能使用磁盘根目录。
            </small>
            <small v-else> 模型权重和物种目录会下载到这里。 </small>
          </label>

          <div class="field-pair">
            <label class="field">
              <span>端口</span>
              <input v-model.number="port" type="number" min="1024" max="65535" />
              <small v-if="!portValid" class="field-error"> 端口范围为 1024–65535。 </small>
            </label>

            <label class="field">
              <span>服务实例名称</span>
              <input v-model="serviceName" autocomplete="off" placeholder="lumen-hub-1" />
              <small v-if="!serviceNameValid" class="field-error">
                以小写字母开头，只能使用小写字母、数字和连字符。
              </small>
            </label>
          </div>
        </fieldset>

        <fieldset>
          <legend>模型能力</legend>

          <div class="model-row">
            <label class="model-check">
              <input v-model="semanticEnabled" type="checkbox" />
              <span>
                <strong>图像语义分析</strong>
                <small>自然语言与相似媒体搜索</small>
              </span>
            </label>
            <select
              v-model="semanticModel"
              :disabled="!semanticEnabled"
              aria-label="图像语义分析模型"
            >
              <option value="siglip2-base-patch16-224">SigLIP 2 Base · 224</option>
              <option value="siglip2-so400m-patch14-384">SigLIP 2 SO400M · 384</option>
            </select>
          </div>

          <label class="model-check standalone">
            <input v-model="faceEnabled" type="checkbox" />
            <span>
              <strong>人物识别</strong>
              <small>InsightFace antelopev2</small>
            </span>
          </label>

          <label class="model-check standalone">
            <input v-model="ocrEnabled" type="checkbox" />
            <span>
              <strong>OCR文字识别</strong>
              <small>PP-OCRv6 small</small>
            </span>
          </label>

          <div class="model-row">
            <label class="model-check">
              <input v-model="bioClipEnabled" type="checkbox" />
              <span>
                <strong>BioCLIP物种识别</strong>
                <small>BioCLIP-2</small>
              </span>
            </label>
            <select
              v-model="bioClipDataset"
              :disabled="!bioClipEnabled"
              aria-label="BioCLIP 物种数据集"
            >
              <option value="TreeOfLife200MCore">Core · 较小目录</option>
              <option value="TreeOfLife200M">Full · 完整目录</option>
            </select>
          </div>

          <small v-if="selectedServiceCount === 0" class="field-error service-error">
            至少选择一项模型能力。
          </small>
        </fieldset>
      </form>

      <div class="config-output">
        <div class="output-actions">
          <span>
            <FileCode2 :size="15" aria-hidden="true" />
            lumen-config.yaml
          </span>
          <div>
            <button type="button" :disabled="!configValid" @click="copyConfig">
              <Check v-if="configCopied" :size="15" aria-hidden="true" />
              <Clipboard v-else :size="15" aria-hidden="true" />
              {{ configCopied ? "已复制" : "复制" }}
            </button>
            <button
              type="button"
              class="primary-action"
              :disabled="!configValid"
              @click="downloadConfig"
            >
              <Download :size="15" aria-hidden="true" />
              下载 YAML
            </button>
          </div>
        </div>
        <pre><code>{{ configYAML }}</code></pre>
      </div>
    </div>

    <div class="launch-builder">
      <header>
        <span class="step-number">2</span>
        <div>
          <h4>选择系统与计算后端</h4>
          <p>计算后端不写入 YAML；CLI 会按下面的 profile 下载对应版本。</p>
        </div>
      </header>

      <div class="launch-grid">
        <form class="launch-form" @submit.prevent>
          <label class="field">
            <span>运行系统</span>
            <select v-model="selectedSystemId">
              <option v-for="system in systems" :key="system.id" :value="system.id">
                {{ system.label }}
              </option>
            </select>
          </label>

          <label class="field">
            <span>计算后端</span>
            <select v-model="selectedBackendId">
              <option
                v-for="backend in selectedSystem.backends"
                :key="backend.id"
                :value="backend.id"
              >
                {{ backend.label }}
              </option>
            </select>
            <small>{{ selectedBackend.note }}</small>
          </label>
        </form>

        <div class="command-output">
          <div class="command-heading">
            <span>
              <Cpu :size="15" aria-hidden="true" />
              {{ selectedBackend.profile }}
            </span>
            <button type="button" @click="copyCommand">
              <Check v-if="commandCopied" :size="15" aria-hidden="true" />
              <Clipboard v-else :size="15" aria-hidden="true" />
              {{ commandCopied ? "已复制" : "复制命令" }}
            </button>
          </div>
          <pre><code>{{ commandText }}</code></pre>
          <p>
            <Play :size="14" aria-hidden="true" />
            把下载的文件放到当前目录并命名为
            <code>lumen-config.yaml</code>，再执行以上命令。
          </p>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.lumen-builder {
  margin: 28px 0;
  overflow: hidden;
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  background: var(--vp-c-bg);
}

.builder-heading {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  padding: 24px;
  border-bottom: 1px solid var(--vp-c-divider);
}

.builder-title {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.builder-heading h3 {
  margin: 6px 0 4px;
  font-size: 22px;
  line-height: 1.25;
}

.builder-heading p,
.launch-builder header p {
  margin: 0;
  color: var(--vp-c-text-2);
  font-size: 13px;
  line-height: 1.55;
}

.builder-kicker,
.fixed-summary {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--vp-c-brand-1);
  font-size: 13px;
  font-weight: 650;
}

.fixed-summary {
  align-self: flex-start;
  color: var(--vp-c-text-2);
  white-space: nowrap;
}

.config-workspace {
  display: grid;
  grid-template-columns: minmax(300px, 0.9fr) minmax(360px, 1.1fr);
  min-height: 720px;
}

.config-form {
  padding: 22px;
  border-right: 1px solid var(--vp-c-divider);
}

.config-form fieldset {
  min-width: 0;
  margin: 0;
  padding: 0;
  border: 0;
}

.config-form fieldset + fieldset {
  margin-top: 24px;
  padding-top: 22px;
  border-top: 1px solid var(--vp-c-divider);
}

.config-form legend {
  margin-bottom: 15px;
  color: var(--vp-c-text-1);
  font-size: 14px;
  font-weight: 700;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 7px;
  color: var(--vp-c-text-1);
  font-size: 13px;
  font-weight: 600;
}

.field + .field {
  margin-top: 15px;
}

.field-pair {
  display: grid;
  grid-template-columns: 0.7fr 1.3fr;
  gap: 12px;
  margin-top: 15px;
}

.field-pair .field {
  margin: 0;
}

.field input,
.field select,
.model-row select {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--vp-c-divider);
  border-radius: 7px;
  padding: 9px 11px;
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg-soft);
  font: inherit;
  font-weight: 450;
}

.field input:focus,
.field select:focus,
.model-row select:focus {
  border-color: var(--vp-c-brand-1);
  outline: 2px solid var(--vp-c-brand-soft);
}

.field small,
.model-check small {
  color: var(--vp-c-text-3);
  font-weight: 450;
  line-height: 1.45;
}

.field-error {
  color: var(--vp-c-danger-1) !important;
}

.model-row,
.model-check.standalone {
  padding: 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
}

.model-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(150px, 0.8fr);
  align-items: center;
  gap: 12px;
}

.model-row + .model-row,
.model-row + .model-check,
.model-check + .model-check,
.model-check + .model-row {
  margin-top: 10px;
}

.model-check {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  cursor: pointer;
}

.model-check input {
  width: 16px;
  height: 16px;
  flex: 0 0 16px;
  margin: 2px 0 0;
  accent-color: var(--vp-c-brand-1);
}

.model-check span,
.model-check strong,
.model-check small {
  display: block;
}

.model-check strong {
  color: var(--vp-c-text-1);
  font-size: 13px;
  line-height: 1.4;
}

.model-check small {
  margin-top: 2px;
  font-size: 11px;
}

.model-row select:disabled {
  cursor: not-allowed;
  opacity: 0.48;
}

.service-error {
  display: block;
  margin-top: 10px;
  font-size: 12px;
}

.config-output {
  min-width: 0;
  background: var(--vp-code-block-bg);
}

.output-actions,
.command-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--vp-c-divider);
  color: var(--vp-c-text-2);
  font-size: 13px;
}

.output-actions > span,
.command-heading > span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.output-actions > div {
  display: flex;
  gap: 8px;
}

.output-actions button,
.command-heading button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 6px;
  padding: 6px 10px;
  color: var(--vp-c-text-1);
  background: var(--vp-c-bg);
  font-weight: 600;
  cursor: pointer;
}

.output-actions button.primary-action {
  border-color: var(--vp-c-brand-1);
  color: var(--vp-c-brand-1);
}

.output-actions button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.config-output pre {
  height: 666px;
  margin: 0;
  padding: 18px;
  overflow: auto;
  border-radius: 0;
  background: transparent;
}

.config-output code,
.command-output code {
  font-size: 12px;
  line-height: 1.55;
}

.launch-builder {
  padding: 24px;
  border-top: 1px solid var(--vp-c-divider);
}

.launch-builder > header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.launch-builder h4 {
  margin: 0 0 3px;
  border: 0;
  font-size: 16px;
}

.step-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  border-radius: 50%;
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
  font-size: 13px;
  font-weight: 750;
}

.launch-grid {
  display: grid;
  grid-template-columns: minmax(240px, 0.7fr) minmax(360px, 1.3fr);
  gap: 18px;
  margin-top: 18px;
}

.launch-form {
  padding: 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 9px;
  background: var(--vp-c-bg-soft);
}

.command-output {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--vp-c-divider);
  border-radius: 9px;
  background: var(--vp-code-block-bg);
}

.command-output pre {
  margin: 0;
  padding: 16px;
  overflow: auto;
  border-radius: 0;
  background: transparent;
}

.command-output p {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  margin: 0;
  padding: 11px 16px;
  border-top: 1px solid var(--vp-c-divider);
  color: var(--vp-c-text-3);
  background: var(--vp-c-bg-soft);
  font-size: 12px;
  line-height: 1.5;
}

.command-output p svg {
  flex: 0 0 auto;
  margin-top: 2px;
}

@media (max-width: 900px) {
  .builder-heading {
    flex-direction: column;
  }

  .config-workspace,
  .launch-grid {
    grid-template-columns: 1fr;
  }

  .config-form {
    border-right: 0;
    border-bottom: 1px solid var(--vp-c-divider);
  }

  .config-output pre {
    height: 520px;
  }
}

@media (max-width: 560px) {
  .builder-heading,
  .config-form,
  .launch-builder {
    padding: 18px;
  }

  .field-pair,
  .model-row {
    grid-template-columns: 1fr;
  }

  .output-actions {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
