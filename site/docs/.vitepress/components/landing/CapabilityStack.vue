<script setup lang="ts">
import { ref } from 'vue'
import { ArrowUpRight, Bird, Bot, Network, ScanFace, ScanSearch, ScanText, Sparkles } from '@lucide/vue'

const activeAgent = ref(0)
const activeSearch = ref(0)

const agentClips = [
  { src: '/videos/landing/agent-organize.webm', label: '整理：按主题归拢与建册' },
  { src: '/videos/landing/agent-curate-multistep.webm', label: '精选：多步筛选与排序' },
  { src: '/videos/landing/agent-in-context-multistep.webm', label: '就地整理：结合上下文的多步操作' },
]

const searchClips = [
  { src: '/videos/landing/search-semantic-easy.webm', label: '语义检索：一句话找到画面' },
  { src: '/videos/landing/search-semantic-multilingual.webm', label: '多语言：中英混合查询' },
  { src: '/videos/landing/search-semantic-hard.webm', label: '复杂语义：细粒度场景召回' },
]
</script>

<template>
  <section class="capability-stack section-shell" aria-label="三项核心能力">
    <article id="agent" class="capability-card capability-agent">
      <div class="capability-copy">
        <div class="capability-number">01 / AGENT</div>
        <Bot :size="34" :stroke-width="1.35" />
        <h2>从一个想法，<br />到一组整理。</h2>
        <p>
          用自然的话去找照片——找人、找文字，甚至某一次光线刚刚好的瞬间。
          Lumilio Agent 不只是搜索，还是会帮忙的整理助手；
          19 个可自由组合的真实工具，不删除、不越界，写操作先确认。
        </p>
        <div class="mode-row" aria-label="Agent 四种模式">
          <span>回顾</span><span>整理</span><span>分析</span><span>精选</span>
        </div>
        <a class="card-link" href="/zh-cn/user-manual/features/lumilio">
          查看 Agent 能力
          <ArrowUpRight :size="18" :stroke-width="1.8" />
        </a>
      </div>
      <div class="agent-proof">
        <div class="capability-stage">
          <video
            :key="`agent-${activeAgent}`"
            :src="agentClips[activeAgent].src"
            :aria-label="agentClips[activeAgent].label"
            autoplay
            loop
            muted
            playsinline
            preload="metadata"
          ></video>
        </div>
        <div class="capability-pager">
          <span class="capability-caption">{{ agentClips[activeAgent].label }}</span>
          <div class="capability-dots" aria-label="切换 Agent 演示">
            <button
              v-for="(clip, index) in agentClips"
              :key="index"
              type="button"
              :class="{ active: activeAgent === index }"
              :aria-label="clip.label"
              @click="activeAgent = index"
            ></button>
          </div>
        </div>
      </div>
      <span class="capability-shade" aria-hidden="true"></span>
    </article>

    <article id="search" class="capability-card capability-search">
      <div class="capability-copy">
        <div class="capability-number">02 / SEARCH</div>
        <ScanSearch :size="34" :stroke-width="1.35" />
        <h2>一个查询，<br />同时读懂画面、文字与地点。</h2>
        <p>
          三路结果并发召回，通过加权 RRF 融合排序。中文使用双字切分，无需额外中文分词插件；
          语义服务离线时，OCR 与地点召回仍会继续工作。
        </p>
        <div class="search-weights">
          <span><b>1.0</b> 语义</span>
          <span><b>0.8</b> 地点</span>
          <span><b>0.7</b> OCR</span>
        </div>
      </div>
      <div class="search-shot">
        <div class="capability-stage">
          <video
            :key="`search-${activeSearch}`"
            :src="searchClips[activeSearch].src"
            :aria-label="searchClips[activeSearch].label"
            autoplay
            loop
            muted
            playsinline
            preload="metadata"
          ></video>
        </div>
        <div class="capability-pager">
          <span class="capability-caption">{{ searchClips[activeSearch].label }}</span>
          <div class="capability-dots" aria-label="切换搜索演示">
            <button
              v-for="(clip, index) in searchClips"
              :key="index"
              type="button"
              :class="{ active: activeSearch === index }"
              :aria-label="clip.label"
              @click="activeSearch = index"
            ></button>
          </div>
        </div>
      </div>
      <span class="capability-shade" aria-hidden="true"></span>
    </article>

    <article id="lumen" class="capability-card capability-lumen">
      <div class="capability-copy">
        <div class="capability-number">03 / LUMEN</div>
        <Network :size="34" :stroke-width="1.35" />
        <h2>局域网里的空闲算力，<br />组成你的本地推理池。</h2>
        <p>
          mDNS 零配置发现，按任务能力选择节点，并在可用节点间轮询。
          受支持的 Desktop 平台可以一键安装本机 Hub；不用 AI，流明集也照常工作。
        </p>
        <a class="card-link" href="/zh-cn/user-manual/introduction/lumen">
          认识 Lumen
          <ArrowUpRight :size="18" :stroke-width="1.8" />
        </a>
      </div>
      <div class="lumen-visual" aria-label="Lumen 本地推理节点示意">
        <div class="node node-hub">
          <span class="hub-pulse" aria-hidden="true"></span>
          <img src="/logo.png" alt="流明集图标" width="512" height="566" loading="lazy" />
          <strong>流明集</strong>
        </div>

        <div class="orbit orbit-pc">
          <span class="orbit-ring" aria-hidden="true"></span>
          <div class="node node-pc">
            <span class="node-icon">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path fill="currentColor" d="M21 16H3V4h18m0-2H3c-1.11 0-2 .89-2 2v12a2 2 0 0 0 2 2h7v2H8v2h8v-2h-2v-2h7a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2" />
              </svg>
            </span>
            <div class="node-body">
              <span>PC · 主机</span>
              <div class="node-models">
                <b><Sparkles :size="12" :stroke-width="2" /> SigLIP 2</b>
                <b><Bird :size="12" :stroke-width="2" /> BioCLIP 2</b>
              </div>
            </div>
          </div>
        </div>

        <div class="orbit orbit-nas">
          <span class="orbit-ring" aria-hidden="true"></span>
          <div class="node node-nas">
            <span class="node-icon">
              <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
                <path fill="currentColor" d="M4 5c-1.11 0-2 .89-2 2v10c0 1.11.89 2 2 2h16c1.11 0 2-.89 2-2V7c0-1.11-.89-2-2-2zm.5 2a1 1 0 0 1 1 1a1 1 0 0 1-1 1a1 1 0 0 1-1-1a1 1 0 0 1 1-1M7 7h13v10H7zm1 1v8h3V8zm4 0v8h3V8zm4 0v8h3V8zM9 9h1v1H9zm4 0h1v1h-1zm4 0h1v1h-1z" />
              </svg>
            </span>
            <div class="node-body">
              <span>NAS · 存储</span>
              <div class="node-models"><b><ScanText :size="12" :stroke-width="2" /> PP-OCRv6</b></div>
            </div>
          </div>
        </div>

        <div class="orbit orbit-sbc">
          <span class="orbit-ring" aria-hidden="true"></span>
          <div class="node node-sbc">
            <span class="node-icon node-icon-pi">
              <svg viewBox="0 0 128 128" aria-hidden="true">
                <path fill="#050606" d="M40.666.002c-.657.02-1.364.26-2.167.883C36.532.138 34.626-.12 32.92 1.4c-2.633-.337-3.488.358-4.137 1.168c-.577-.012-4.324-.586-6.042 1.94c-4.317-.504-5.683 2.5-4.137 5.303c-.881 1.345-1.796 2.673.266 5.236c-.728 1.428-.276 2.976 1.443 4.852c-.454 2.007.437 3.422 2.036 4.526c-.3 2.746 2.557 4.344 3.41 4.912c.327 1.6 1.01 3.111 4.273 3.945c.537 2.387 2.499 2.798 4.397 3.298c-6.275 3.594-11.657 8.32-11.62 19.92l-.92 1.615c-7.195 4.31-13.669 18.162-3.546 29.422c.662 3.525 1.77 6.056 2.758 8.858c1.477 11.291 11.115 16.577 13.657 17.203c3.726 2.794 7.693 5.445 13.062 7.303c5.06 5.142 10.544 7.101 16.058 7.099h.243c5.513.003 10.997-1.957 16.057-7.099c5.37-1.857 9.336-4.509 13.061-7.303c2.543-.626 12.18-5.912 13.657-17.204c.987-2.801 2.097-5.332 2.759-8.857c10.123-11.26 3.649-25.114-3.547-29.425l-.92-1.614c.037-11.598-5.345-16.325-11.62-19.92c1.898-.5 3.86-.911 4.398-3.297c3.261-.835 3.944-2.345 4.271-3.945c.854-.57 3.71-2.166 3.41-4.914c1.6-1.102 2.491-2.519 2.038-4.525c1.718-1.875 2.17-3.424 1.44-4.851c2.064-2.562 1.148-3.891.267-5.236c1.546-2.802.183-5.807-4.137-5.304c-1.718-2.524-5.464-1.95-6.042-1.94c-.649-.81-1.504-1.504-4.137-1.167c-1.704-1.52-3.611-1.26-5.578-.514c-2.334-1.814-3.88-.36-5.645.19c-2.827-.91-3.473.337-4.862.844c-3.083-.642-4.02.755-5.498 2.23l-1.72-.033c-4.649 2.699-6.96 8.195-7.777 11.02c-.82-2.826-3.124-8.322-7.773-11.02l-1.72.032c-1.48-1.475-2.417-2.871-5.5-2.229c-1.388-.507-2.033-1.754-4.862-.844c-1.159-.36-2.224-1.112-3.478-1.074l.002.001" />
                <path fill="#63c54d" d="M31.501 11.878c12.337 6.264 19.508 11.333 23.437 15.649c-2.011 7.943-12.508 8.306-16.347 8.082c.786-.36 1.443-.792 1.675-1.453c-.963-.675-4.378-.072-6.762-1.392c.915-.187 1.344-.369 1.772-1.034c-2.253-.708-4.678-1.318-6.106-2.49c.77.01 1.49.17 2.495-.518c-2.018-1.07-4.17-1.919-5.843-3.556c1.042-.025 2.168-.01 2.495-.388c-1.847-1.126-3.406-2.38-4.694-3.75c1.46.174 2.076.024 2.43-.228c-1.398-1.407-3.164-2.596-4.006-4.331c1.084.369 2.076.51 2.79-.033c-.475-1.054-2.506-1.676-3.677-4.138c1.141.109 2.352.245 2.594 0c-.53-2.126-1.438-3.32-2.33-4.558c2.442-.036 6.142.009 5.975-.195l-1.51-1.519c2.385-.632 4.826.102 6.598.647c.795-.619-.014-1.4-.985-2.2c2.028.268 3.859.728 5.514 1.359c.885-.787-.574-1.573-1.28-2.36c3.133.585 4.46 1.407 5.777 2.23c.958-.903.055-1.67-.59-2.456c2.362.861 3.578 1.974 4.859 3.07c.434-.576 1.102-1 .295-2.392c1.676.952 2.94 2.074 3.872 3.33c1.038-.65.619-1.54.625-2.36c1.742 1.397 2.849 2.882 4.202 4.333c.272-.195.51-.859.722-1.908c4.157 3.972 10.03 13.978 1.51 17.945c-7.252-5.89-15.913-10.173-25.51-13.386h.002m65.344 0C84.507 18.143 77.336 23.21 73.407 27.527c2.012 7.943 12.51 8.306 16.347 8.082c-.786-.36-1.442-.792-1.674-1.453c.964-.675 4.378-.072 6.763-1.392c-.916-.187-1.346-.369-1.773-1.034c2.252-.708 4.679-1.318 6.105-2.49c-.77.01-1.49.17-2.495-.518c2.018-1.07 4.17-1.919 5.844-3.556c-1.044-.025-2.168-.01-2.495-.388c1.847-1.126 3.405-2.38 4.694-3.75c-1.46.174-2.076.024-2.43-.228c1.397-1.407 3.164-2.596 4.006-4.331c-1.084.369-2.076.51-2.79-.033c.474-1.054 2.505-1.676 3.677-4.138c-1.142.109-2.352.245-2.595 0c.532-2.126 1.44-3.321 2.331-4.56c-2.442-.035-6.142.01-5.975-.193l1.512-1.519c-2.387-.633-4.828.1-6.599.645c-.796-.618.014-1.399.984-2.198c-2.026.267-3.859.726-5.514 1.358c-.885-.787.574-1.573 1.28-2.36c-3.132.585-4.458 1.407-5.777 2.23c-.957-.903-.054-1.67.59-2.456c-2.362.861-3.578 1.974-4.858 3.07c-.433-.576-1.103-1-.296-2.392c-1.676.952-2.94 2.074-3.872 3.33c-1.038-.651-.619-1.54-.625-2.36c-1.742 1.397-2.849 2.883-4.201 4.333c-.273-.195-.511-.86-.723-1.908c-4.156 3.972-10.03 13.978-1.51 17.945c7.249-5.892 15.908-10.174 25.507-13.386h-.001" />
                <path fill="#c51850" d="M79.092 92.768c.043 7.412-6.539 13.453-14.7 13.492s-14.811-5.938-14.855-13.351v-.141c-.043-7.414 6.538-13.455 14.7-13.494s14.812 5.938 14.855 13.351v.141m-23.041-38.34c6.123 3.95 7.227 12.908 2.466 20.004s-13.586 9.648-19.709 5.696c-6.122-3.95-7.227-12.909-2.466-20.005c4.762-7.097 13.586-9.648 19.709-5.696m16.527-.716c-6.123 3.952-7.227 12.909-2.465 20.006s13.585 9.648 19.707 5.695c6.124-3.95 7.228-12.907 2.466-20.005c-4.762-7.096-13.584-9.646-19.708-5.695m-46.751 7.216c6.61-1.745 2.231 26.94-3.147 24.586c-5.917-4.687-7.823-18.416 3.146-24.586m76.398-.357c-6.611-1.745-2.232 26.94 3.147 24.587c5.917-4.688 7.822-18.417-3.147-24.587M80.052 39.167c11.408-1.898 20.9 4.778 20.518 16.964c-.375 4.671-24.721-16.269-20.518-16.965m-31.521-.357c-11.41-1.898-20.903 4.78-20.52 16.966c.376 4.67 24.722-16.27 20.52-16.966m15.716-2.842c-6.809-.173-13.343 4.98-13.36 7.966c-.018 3.632 5.384 7.35 13.408 7.444c8.192.057 13.42-2.975 13.447-6.723c.029-4.246-7.453-8.752-13.495-8.687m.526 74.462c5.937-.256 13.904 1.883 13.919 4.72c.099 2.755-7.225 8.98-14.312 8.86c-7.34.312-14.538-5.922-14.444-8.083c-.11-3.169 8.939-5.642 14.837-5.497m-21.97-16.815c4.226 5.017 6.153 13.828 2.626 16.425c-3.336 1.984-11.44 1.167-17.202-6.984c-3.883-6.838-3.383-13.798-.655-15.842c4.079-2.448 10.381.858 15.23 6.4m42.557-1.589c-4.575 5.277-7.122 14.9-3.785 17.999c3.19 2.408 11.752 2.071 18.078-6.574c4.593-5.806 3.054-15.501.43-18.076c-3.897-2.97-9.49.83-14.724 6.65v.002" />
              </svg>
            </span>
            <div class="node-body">
              <span>SBC · 树莓派</span>
              <div class="node-models"><b><ScanFace :size="12" :stroke-width="2" /> InsightFace</b></div>
            </div>
          </div>
        </div>
      </div>
      <span class="capability-shade" aria-hidden="true"></span>
    </article>
  </section>
</template>

<style>
.capability-stack {
  padding-top: 170px;
  padding-bottom: 220px;
}

.capability-card {
  position: sticky;
  top: var(--capability-top);
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr);
  gap: clamp(40px, 4.5vw, 72px);
  height: var(--capability-height);
  min-height: 0;
  margin-bottom: 150px;
  padding: clamp(34px, 4vw, 58px);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 34px;
  color: #f2efe8;
  box-shadow: 0 45px 110px rgba(24, 22, 17, 0.24);
  transform-origin: center top;
}

.capability-card > :not(.capability-shade) {
  position: relative;
  z-index: 1;
}

.capability-shade {
  position: absolute;
  z-index: 10;
  inset: 0;
  border-radius: inherit;
  background: #080a08;
  opacity: 0;
  pointer-events: none;
}

.capability-agent {
  z-index: 1;
  background: #12130f;
}

.capability-search {
  z-index: 2;
  background: #101824;
}

.capability-lumen {
  z-index: 3;
  margin-bottom: 0;
  background: #172116;
}

.capability-copy {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  min-height: 0;
}

.capability-card .capability-number {
  color: #aaa89f;
}

.capability-card h2 {
  max-width: 680px;
  margin: clamp(24px, 3svh, 36px) 0 clamp(18px, 2.4svh, 26px);
  color: #f2efe8;
  font-size: clamp(40px, 4.2vw, 64px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.055em;
}

.capability-card .capability-copy > p {
  max-width: 610px;
  margin: 0;
  color: #b9b5ac;
  font-size: clamp(16px, 1.25vw, 19px);
  line-height: 1.62;
}

.mode-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: clamp(20px, 3svh, 30px);
}

.mode-row span {
  padding: 7px 12px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 8px;
  color: #d1cec6;
  font-size: 13px;
}

.capability-card .card-link {
  margin-top: auto;
  padding-top: clamp(20px, 3svh, 34px);
  color: var(--acid);
}

.agent-proof,
.search-shot,
.lumen-visual {
  align-self: center;
  width: 100%;
  min-width: 0;
  min-height: 0;
  max-height: 100%;
}

.lumen-visual {
  aspect-ratio: 16 / 10;
}

.agent-proof,
.search-shot {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border-radius: 22px;
}

.agent-proof {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: #1c1d19;
}

.capability-stage {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  border-radius: 14px;
  background: #0b0d0b;
  aspect-ratio: 3 / 2;
}

.capability-stage video {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
}

.capability-pager {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.capability-caption {
  min-width: 0;
  overflow: hidden;
  color: #b9b5ac;
  font-size: 12px;
  line-height: 1.4;
  letter-spacing: 0.01em;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.capability-dots {
  display: flex;
  flex: 0 0 auto;
  gap: 8px;
}

.capability-dots button {
  appearance: none;
  width: 30px;
  height: 3px;
  border: 0;
  background: rgba(255, 255, 255, 0.28);
  cursor: pointer;
  transition: background 180ms ease, width 180ms ease;
}

.capability-dots button.active {
  width: 60px;
  background: var(--acid);
}

.capability-search .capability-dots button.active {
  background: var(--blue);
}

.lumen-visual {
  position: relative;
  overflow: hidden;
  container-type: size;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 28px;
  background: #0f160e;
}

/* Each orbit is a hub-centred square arm that slowly rotates; its node rides
   the top of the square and counter-rotates to stay upright. */
.orbit {
  position: absolute;
  left: 50%;
  top: 50%;
  z-index: 2;
  width: calc(var(--radius) * 2);
  height: calc(var(--radius) * 2);
  margin-top: calc(var(--radius) * -1);
  margin-left: calc(var(--radius) * -1);
  pointer-events: none;
  animation: orbit-spin var(--dur) linear var(--delay, 0s) infinite;
}

.orbit-ring {
  position: absolute;
  inset: 0;
  border: 1px dashed rgb(var(--accent) / 0.28);
  border-radius: 50%;
}

/* Same period, thirds-of-a-turn apart → constant 120° spacing, never clumping. */
.orbit-pc { --accent: 122 178 240; --radius: 35cqh; --dur: 54s; --delay: 0s; }
.orbit-sbc { --accent: 240 105 154; --radius: 40cqh; --dur: 54s; --delay: -18s; }
.orbit-nas { --accent: 232 185 96; --radius: 30cqh; --dur: 54s; --delay: -36s; }

.node {
  position: absolute;
  top: 0;
  left: 50%;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 176px;
  padding: 13px 15px;
  border: 1px solid rgb(var(--accent, 200 241 105) / 0.26);
  border-radius: 16px;
  background: rgba(26, 37, 23, 0.92);
  box-shadow: 0 18px 42px rgba(6, 12, 5, 0.42);
  backdrop-filter: blur(3px);
  pointer-events: auto;
  transition: border-color 220ms ease, box-shadow 220ms ease;
  animation: orbit-upright var(--dur) linear var(--delay, 0s) infinite;
}

.node:hover {
  border-color: rgb(var(--accent) / 0.6);
  box-shadow: 0 22px 50px rgba(6, 12, 5, 0.52);
}

.node-pc { width: 214px; }

.node-icon {
  display: grid;
  flex: 0 0 auto;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 11px;
  background: rgb(var(--accent) / 0.16);
  color: rgb(var(--accent));
}

.node-icon svg {
  display: block;
  width: 24px;
  height: 24px;
}

.node-icon-pi {
  background: rgba(255, 255, 255, 0.06);
}

.node-icon-pi svg {
  width: 26px;
  height: 26px;
}

.node-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.node-body > span {
  color: #93a487;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.node-models {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.node-models b {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 7px;
  border-radius: 7px;
  background: rgb(var(--accent) / 0.14);
  color: #eef2e6;
  font-size: 12px;
  font-weight: 500;
}

.node-hub {
  left: 50%;
  top: 50%;
  z-index: 3;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
  width: 168px;
  height: 168px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: var(--acid);
  color: #12140e;
  text-align: center;
  box-shadow: 0 0 0 1px rgba(200, 241, 105, 0.55), 0 26px 64px rgba(110, 150, 36, 0.4);
  transform: translate(-50%, -50%);
  animation: none;
}

.hub-pulse {
  position: absolute;
  inset: 0;
  border: 1.5px solid var(--acid);
  border-radius: inherit;
  opacity: 0;
  pointer-events: none;
  animation: hub-pulse 2.8s ease-out infinite;
}

.node-hub img {
  width: 52px;
  height: 52px;
  object-fit: contain;
}

.node-hub strong {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
}

@keyframes orbit-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

@keyframes orbit-upright {
  from { transform: translate(-50%, -50%) rotate(0deg); }
  to { transform: translate(-50%, -50%) rotate(-360deg); }
}

@keyframes hub-pulse {
  0% { opacity: 0.5; transform: scale(1); }
  70% { opacity: 0; }
  100% { opacity: 0; transform: scale(1.5); }
}

@media (prefers-reduced-motion: reduce) {
  .orbit,
  .orbit .node,
  .hub-pulse { animation-play-state: paused; }
}

.search-weights {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  width: 100%;
  margin-top: auto;
  padding-top: 45px;
}

.search-weights span {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: #9da9b9;
  font-size: 12px;
}

.search-weights b {
  color: var(--blue);
  font-size: clamp(28px, 3vw, 47px);
  font-weight: 500;
}

.search-shot {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: #101821;
}

@media (min-width: 1101px) and (max-height: 820px) {
  .lumilio-landing {
    --capability-top: 90px;
    --capability-height: calc(100svh - var(--capability-top) - 18px);
  }

  .capability-stack { padding-top: 110px; padding-bottom: 170px; }
  .capability-card { gap: 34px; margin-bottom: 96px; padding: 28px 32px; }
  .capability-card .capability-number { margin-bottom: 16px; }
  .capability-card h2 { margin: 20px 0 15px; font-size: clamp(35px, 3.7vw, 54px); }
  .capability-card .capability-copy > p { font-size: 15px; line-height: 1.5; }
  .capability-card .card-link { padding-top: 18px; }
  .mode-row { gap: 7px; margin-top: 17px; }
  .mode-row span { padding: 5px 9px; font-size: 11px; }
  .agent-proof,
  .search-shot { gap: 10px; padding: 12px; }
  .node { width: 158px; padding: 11px 13px; }
  .node-pc { width: 196px; }
  .node-icon { width: 36px; height: 36px; }
  .node-hub { width: 150px; height: 150px; }
  .node-hub img { width: 46px; height: 46px; }
}

@media (max-height: 700px) {
  .capability-card {
    position: relative;
    top: auto;
    height: auto;
    min-height: 0;
    transform: none !important;
  }

  .capability-shade { display: none; }
  .agent-proof,
  .search-shot,
  .lumen-visual { max-height: none; }
}

@media (max-width: 1100px) {
  .capability-card { grid-template-columns: 1fr; position: relative; top: auto; height: auto; min-height: 0; transform: none !important; }
  .capability-shade { display: none; }
  .capability-card .card-link { margin-top: 35px; padding-top: 0; }
  .agent-proof,
  .search-shot,
  .lumen-visual { max-height: none; }
}

@media (max-width: 760px) {
  .capability-stack { padding-top: 100px; padding-bottom: 130px; }
  .capability-card { gap: 50px; margin-bottom: 70px; padding: 28px 20px; border-radius: 24px; }
  .capability-card h2 { margin-top: 32px; font-size: clamp(40px, 12vw, 58px); }
  .capability-card .capability-copy > p { font-size: 16px; }
  .agent-proof,
  .search-shot { height: auto; padding: 12px; aspect-ratio: auto; }
  /* Orbiting wide cards can't fit a narrow phone frame — stack them instead. */
  .lumen-visual {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 14px;
    height: auto;
    padding: 30px 16px;
    aspect-ratio: auto;
    container-type: normal;
  }
  .node-hub { order: -1; position: static; width: 132px; height: 132px; transform: none; }
  .node-hub img { width: 44px; height: 44px; }
  .node-hub strong { font-size: 14px; }
  .orbit {
    position: static;
    width: 100%;
    max-width: 320px;
    height: auto;
    margin: 0;
    animation: none;
  }
  .orbit-ring { display: none; }
  .node {
    position: static;
    width: 100%;
    padding: 12px 14px;
    transform: none;
    animation: none;
  }
  .node-pc { width: 100%; }
  .search-weights { gap: 14px; margin-top: 36px; padding-top: 0; }
}
</style>
