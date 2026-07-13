<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  ArrowRight,
  ArrowUpRight,
  Bot,
  Check,
  ChevronLeft,
  ChevronRight,
  GitFork,
  HardDrive,
  Network,
  ScanSearch,
  ShieldCheck,
} from '@lucide/vue'

const activeProof = ref(0)

const proofs = [
  {
    kicker: '真实资源库',
    title: '不是样机，是跑过 Lumen 的照片库。',
    text: '25 张公开照片，补齐时间、地点、相机与镜头信息，用来覆盖真实检索与整理路径。',
    image: '/images/landing/product-library.png',
    alt: 'Lumilio Photos 真实资源库截图',
  },
  {
    kicker: '融合搜索',
    title: '一句 Tokyo coffee，同时找到海报与咖啡馆。',
    text: '画面语义、照片文字、拍摄地点并行召回，再由加权 RRF 合并成一份结果。',
    image: '/images/landing/product-search.png',
    alt: 'Lumilio Photos 搜索 Tokyo coffee 的真实截图',
  },
  {
    kicker: '人物聚类',
    title: '同一个人，跨近景、色调与构图重逢。',
    text: 'InsightFace 提供本地人脸能力；纠错与合并结果会在重新聚类后继续保留。',
    image: '/images/landing/product-people.png',
    alt: 'Lumilio Photos 人物聚类真实截图',
  },
  {
    kicker: '时空地图',
    title: '照片重新回到它发生的地方。',
    text: '拍摄统计与 GPS 地图把散落的旅行变成可浏览的时间和空间线索。',
    image: '/images/landing/product-map.png',
    alt: 'Lumilio Photos 时空地图真实截图',
  },
]

const dailyFeatures = [
  ['01', '自动堆叠', 'Live Photo、RAW + JPEG、连拍与编辑版本'],
  ['02', '感知查重', '精确哈希与 64 位 DCT pHash'],
  ['03', '人物纠错', '命名、拆分、合并并穿越重新聚类'],
  ['04', '智能分类', '文档、票据、插画等零样本分类'],
  ['05', '工作室边框', '五种版式，EXIF 仅在导出时烘焙'],
  ['06', '安全分享', '32 字节随机令牌，服务端只存 HMAC'],
  ['07', '拍摄统计', '相机、镜头、焦段与时间趋势'],
  ['08', '时空地图', '用 GPS 重新浏览旅程'],
]

let animationContext: { revert: () => void } | undefined

function previousProof() {
  activeProof.value = (activeProof.value - 1 + proofs.length) % proofs.length
}

function nextProof() {
  activeProof.value = (activeProof.value + 1) % proofs.length
}

function jumpTo(sectionId: string) {
  document.getElementById(sectionId)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.replaceState(null, '', `#${sectionId}`)
}

onMounted(async () => {
  document.documentElement.classList.add('lumilio-landing-page')
  await nextTick()

  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

  const [{ gsap }, { ScrollTrigger }] = await Promise.all([
    import('gsap'),
    import('gsap/ScrollTrigger'),
  ])
  gsap.registerPlugin(ScrollTrigger)

  animationContext = gsap.context(() => {
    gsap.from('.hero-copy > *', {
      y: 44,
      opacity: 0,
      duration: 1.05,
      stagger: 0.1,
      ease: 'power3.out',
    })

    gsap.from('.hero-stage', {
      y: 70,
      opacity: 0,
      scale: 0.96,
      duration: 1.25,
      delay: 0.18,
      ease: 'power3.out',
    })

    gsap.utils.toArray<HTMLElement>('.reveal-image').forEach((image) => {
      gsap.fromTo(
        image,
        { opacity: 0, scale: 1.08 },
        {
          opacity: 1,
          scale: 1,
          ease: 'none',
          scrollTrigger: {
            trigger: image,
            start: 'top 88%',
            end: 'top 42%',
            scrub: 0.8,
          },
        },
      )
    })

    const cards = gsap.utils.toArray<HTMLElement>('.capability-card')
    cards.slice(0, -1).forEach((card, index) => {
      gsap.to(card, {
        scale: 0.94 - index * 0.01,
        y: -26,
        filter: 'brightness(0.52)',
        ease: 'none',
        scrollTrigger: {
          trigger: cards[index + 1],
          start: 'top 82%',
          end: 'top 24%',
          scrub: true,
        },
      })
    })

    gsap.utils.toArray<HTMLElement>('.section-heading').forEach((heading) => {
      gsap.from(heading, {
        y: 60,
        opacity: 0,
        duration: 0.9,
        ease: 'power3.out',
        scrollTrigger: {
          trigger: heading,
          start: 'top 84%',
          once: true,
        },
      })
    })
  })
})

onBeforeUnmount(() => {
  animationContext?.revert()
  document.documentElement.classList.remove('lumilio-landing-page')
})
</script>

<template>
  <div class="lumilio-landing">
    <header class="landing-nav" aria-label="主导航">
      <a class="brand" href="/zh-cn/" aria-label="Lumilio Photos 中文首页">
        <img src="/logo.png" alt="Lumilio Photos 图标" />
        <span>Lumilio Photos</span>
      </a>
      <nav class="nav-links" aria-label="页面章节">
        <a href="#agent" @click.prevent="jumpTo('agent')">Agent</a>
        <a href="#lumen" @click.prevent="jumpTo('lumen')">Lumen</a>
        <a href="#search" @click.prevent="jumpTo('search')">搜索</a>
        <a href="#integrity" @click.prevent="jumpTo('integrity')">完整性</a>
      </nav>
      <a class="nav-action" href="/zh-cn/user-manual/introduction/installation">
        开始使用
        <ArrowUpRight :size="17" :stroke-width="1.8" />
      </a>
    </header>

    <main>
      <section class="hero section-shell">
        <div class="hero-copy">
          <p class="eyebrow">本地优先的照片系统</p>
          <h1>照片，重新<br />回到你手里。</h1>
          <p class="hero-lede">
            原生照片 Agent、本地推理池与三路融合搜索。
            原件仍是普通文件，AI 永远可选。
          </p>
          <div class="hero-actions">
            <a class="button button-primary" href="/zh-cn/user-manual/introduction/installation">
              安装 Lumilio
              <ArrowRight :size="19" :stroke-width="1.8" />
            </a>
            <a class="text-link" href="/zh-cn/user-manual/introduction/">
              阅读用户手册
              <ArrowUpRight :size="17" :stroke-width="1.8" />
            </a>
          </div>
          <div class="hero-meta" aria-label="项目特性">
            <span>开源免费</span>
            <span>普通目录仓库</span>
            <span>macOS · Windows · Web</span>
          </div>
        </div>

        <div class="hero-stage" aria-label="Lumilio Photos 真实应用场景">
          <div class="hero-stage-bar">
            <span class="live-dot"></span>
            <span>真实运行 · 25 张演示照片</span>
            <span class="hero-stage-index">LIBRARY / 01</span>
          </div>
          <div class="hero-screen">
            <img
              class="reveal-image"
              src="/images/landing/product-library.png"
              alt="Lumilio Photos 真实资源库界面"
            />
          </div>
          <figure class="floating-photo floating-photo-left">
            <img src="/images/landing/scene-lake.webp" alt="湖边木屋照片" />
          </figure>
          <figure class="floating-photo floating-photo-right">
            <img src="/images/landing/scene-portrait.webp" alt="人物肖像照片" />
          </figure>
        </div>
      </section>

      <section class="statement section-shell">
        <p class="section-kicker">不止是一个图库</p>
        <h2 class="section-heading">
          让照片理解照片，
          <span class="inline-photo"><img src="/images/landing/scene-fox.webp" alt="红狐照片" /></span>
          也让你保有离开的权利。
        </h2>
        <p class="statement-copy">
          Lumilio 把智能能力放进真实工作流，而不是把媒体锁进黑箱。
          查找、整理、精选、分享都可以更快；原件、目录和退出权仍然属于你。
        </p>
      </section>

      <section class="marquee" aria-label="Lumilio 能力索引">
        <div class="marquee-track">
          <span>SEMANTIC</span><i></i><span>OCR</span><i></i><span>PLACES</span><i></i>
          <span>INSIGHTFACE</span><i></i><span>SIGLIP 2</span><i></i><span>BIOCLIP 2</span><i></i>
          <span>AGENT TOOLS</span><i></i><span>SEMANTIC</span><i></i><span>OCR</span><i></i>
          <span>PLACES</span><i></i><span>INSIGHTFACE</span><i></i><span>SIGLIP 2</span><i></i>
          <span>BIOCLIP 2</span><i></i><span>AGENT TOOLS</span><i></i>
        </div>
      </section>

      <section class="capability-stack section-shell" aria-label="三项核心能力">
        <article id="agent" class="capability-card capability-agent">
          <div class="capability-copy">
            <div class="capability-number">01 / AGENT</div>
            <Bot :size="34" :stroke-width="1.35" />
            <h2>不是聊天框。<br />是会把照片工作做完的 Agent。</h2>
            <p>
              回顾、整理、分析、精选四种模式；可提及人物、相册、看板、相机与镜头。
              背后连接 19 个真实工具，从筛选、画质排序、pHash 去重，到建相册与打标签。
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
            <div class="agent-command">
              <span class="command-label">精选模式</span>
              <p>从 2025 年旅行与自然照片里精选 6 张，按画质排序，折叠连拍和近似重复。</p>
            </div>
            <div class="tool-chain" aria-label="真实 Agent 工具链">
              <div><span>search_semantic</span><b>8</b></div>
              <div><span>filter_assets · 2025</span><b>25</b></div>
              <div><span>rank · SigLIP quality</span><b>8</b></div>
              <div><span>dedupe · pHash</span><b>6</b></div>
            </div>
            <figure class="confirm-shot">
              <img
                class="reveal-image"
                src="/images/landing/product-agent-confirm-crop.webp"
                alt="Lumilio Agent 创建相册前请求确认的真实截图"
              />
              <figcaption>真实操作：创建相册并加入 6 张照片前，先展示影响范围并确认。</figcaption>
            </figure>
          </div>
        </article>

        <article id="lumen" class="capability-card capability-lumen">
          <div class="capability-copy">
            <div class="capability-number">02 / LUMEN</div>
            <Network :size="34" :stroke-width="1.35" />
            <h2>局域网里的空闲算力，<br />组成你的本地推理池。</h2>
            <p>
              mDNS 零配置发现，按任务能力选择节点，并在可用节点间轮询。
              受支持的 Desktop 平台可以一键安装本机 Hub；不用 AI，Lumilio 也照常工作。
            </p>
            <a class="card-link" href="/zh-cn/user-manual/introduction/lumen">
              认识 Lumen
              <ArrowUpRight :size="18" :stroke-width="1.8" />
            </a>
          </div>
          <div class="lumen-visual" aria-label="Lumen 本地推理节点示意">
            <div class="node node-hub">
              <img src="/logo.png" alt="Lumilio Photos 图标" />
              <strong>Lumen Hub</strong>
              <span>LOCAL · READY</span>
            </div>
            <div class="node node-one"><span>MAC STUDIO</span><b>SigLIP 2</b></div>
            <div class="node node-two"><span>DESKTOP</span><b>PP-OCRv6</b></div>
            <div class="node node-three"><span>LAPTOP</span><b>InsightFace</b></div>
            <div class="node node-four"><span>WORKSTATION</span><b>BioCLIP 2</b></div>
            <svg class="node-lines" viewBox="0 0 800 620" role="presentation" aria-hidden="true">
              <path d="M400 310 L180 150 M400 310 L620 150 M400 310 L180 470 M400 310 L620 470" />
            </svg>
          </div>
        </article>

        <article id="search" class="capability-card capability-search">
          <div class="capability-copy">
            <div class="capability-number">03 / SEARCH</div>
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
          <figure class="search-shot">
            <img
              class="reveal-image"
              src="/images/landing/product-search.png"
              alt="Lumilio Photos 多路融合搜索真实截图"
            />
            <figcaption>
              <span>真实查询</span>
              Tokyo coffee
            </figcaption>
          </figure>
        </article>
      </section>

      <section class="proof-section section-shell">
        <div class="proof-header">
          <div>
            <p class="section-kicker">真实工作流证据</p>
            <h2 class="section-heading">每张截图，<br />都来自正在运行的 Lumilio。</h2>
          </div>
          <div class="proof-controls" aria-label="切换真实截图">
            <button type="button" aria-label="上一张截图" @click="previousProof">
              <ChevronLeft :size="20" :stroke-width="1.8" />
            </button>
            <span>{{ String(activeProof + 1).padStart(2, '0') }} / 04</span>
            <button type="button" aria-label="下一张截图" @click="nextProof">
              <ChevronRight :size="20" :stroke-width="1.8" />
            </button>
          </div>
        </div>

        <div class="proof-frame">
          <div class="proof-copy">
            <p>{{ proofs[activeProof].kicker }}</p>
            <h3>{{ proofs[activeProof].title }}</h3>
            <span>{{ proofs[activeProof].text }}</span>
            <div class="proof-dots" aria-label="截图页码">
              <button
                v-for="(_, index) in proofs"
                :key="index"
                type="button"
                :class="{ active: activeProof === index }"
                :aria-label="`查看第 ${index + 1} 张截图`"
                @click="activeProof = index"
              ></button>
            </div>
          </div>
          <div class="proof-image-wrap">
            <Transition name="proof-fade" mode="out-in">
              <img
                :key="proofs[activeProof].image"
                :src="proofs[activeProof].image"
                :alt="proofs[activeProof].alt"
              />
            </Transition>
          </div>
        </div>
      </section>

      <section id="integrity" class="integrity-section section-shell">
        <p class="section-kicker">信任不是口号，是数据路径</p>
        <h2 class="section-heading">
          AI 可以离线，
          <span class="inline-photo inline-photo-tall"><img src="/images/landing/scene-camera.webp" alt="相机照片" /></span>
          原件不能失控。
        </h2>

        <div class="integrity-grid">
          <article class="integrity-card integrity-primary">
            <div class="integrity-icon"><HardDrive :size="25" :stroke-width="1.5" /></div>
            <div>
              <p>普通目录就是仓库</p>
              <h3>随时看得见，随时可以离开。</h3>
              <span>移除仓库不会删除目录与媒体；原件仍保存在你能直接访问的位置。</span>
            </div>
            <img class="reveal-image" src="/images/landing/scene-lake.webp" alt="湖边木屋照片" />
          </article>
          <article class="integrity-card integrity-secondary">
            <div class="integrity-icon"><ShieldCheck :size="25" :stroke-width="1.5" /></div>
            <div>
              <p>写入前先站稳</p>
              <h3>BLAKE3 指纹、暂存后提交、软删除。</h3>
              <span>导入路径先校验再提交；移除记录不会立刻碰原件。</span>
            </div>
          </article>
          <article class="integrity-card integrity-tertiary">
            <div class="integrity-icon"><Check :size="25" :stroke-width="1.5" /></div>
            <div>
              <p>编辑不覆盖原件</p>
              <h3>侧车保存调整，数据库每天自动备份。</h3>
              <span>默认每 24 小时备份数据库，保留最近 14 份。</span>
            </div>
          </article>
        </div>
      </section>

      <section class="daily-section section-shell">
        <div class="daily-heading">
          <p class="section-kicker">每天都用得上的底盘</p>
          <h2 class="section-heading">聪明，但不喧宾夺主。</h2>
        </div>
        <div class="daily-list">
          <article v-for="feature in dailyFeatures" :key="feature[0]">
            <span>{{ feature[0] }}</span>
            <h3>{{ feature[1] }}</h3>
            <p>{{ feature[2] }}</p>
            <ArrowUpRight :size="18" :stroke-width="1.55" />
          </article>
        </div>
      </section>

      <section class="local-section section-shell">
        <div class="local-copy">
          <p class="section-kicker">LOCAL FIRST</p>
          <h2 class="section-heading">你的照片库，<br />不该以联网为前提。</h2>
          <p>
            使用本机 Ollama，或连接支持工具调用的 OpenAI 兼容端点；
            Lumen Hub 可留在本机，也可以发现局域网节点。选择权一直在你手里。
          </p>
        </div>
        <div class="local-stats">
          <div><strong>0</strong><span>必需云服务</span></div>
          <div><strong>4</strong><span>本地视觉模型</span></div>
          <div><strong>19</strong><span>Agent 真实工具</span></div>
        </div>
      </section>

      <section class="cta-section section-shell">
        <div class="cta-visual">
          <img class="reveal-image" src="/images/landing/scene-beach.webp" alt="海滩照片" />
          <img class="cta-bird" src="/images/landing/scene-bird.webp" alt="飞鸟照片" />
        </div>
        <div class="cta-copy">
          <p class="section-kicker">现在，把照片带回来</p>
          <h2>从一个普通目录开始。</h2>
          <p>免费、开源。先导入几张照片，再决定哪些智能能力值得打开。</p>
          <div class="cta-actions">
            <a class="button button-dark" href="/zh-cn/user-manual/introduction/installation">
              开始安装
              <ArrowRight :size="19" :stroke-width="1.8" />
            </a>
            <a class="button button-outline" href="/zh-cn/user-manual/introduction/">
              阅读文档
            </a>
            <a
              class="button button-outline"
              href="https://github.com/EdwinZhanCN/Lumilio-Photos"
              target="_blank"
              rel="noreferrer"
            >
              <GitFork :size="18" :stroke-width="1.7" />
              查看源码
            </a>
          </div>
        </div>
      </section>
    </main>

    <footer class="landing-footer section-shell">
      <div class="brand footer-brand">
        <img src="/logo.png" alt="Lumilio Photos 图标" />
        <span>Lumilio Photos</span>
      </div>
      <p>给照片一个聪明、诚实、随时可以离开的家。</p>
      <div class="footer-links">
        <a href="/zh-cn/user-manual/introduction/">文档</a>
        <a href="/redoc-static.html">API</a>
        <a href="https://github.com/EdwinZhanCN/Lumilio-Photos">GitHub</a>
      </div>
    </footer>
  </div>
</template>

<style>
@font-face {
  font-family: 'Satoshi';
  src: url('/fonts/satoshi-400.woff2') format('woff2');
  font-style: normal;
  font-weight: 400;
  font-display: swap;
}

@font-face {
  font-family: 'Satoshi';
  src: url('/fonts/satoshi-500.woff2') format('woff2');
  font-style: normal;
  font-weight: 500;
  font-display: swap;
}

@font-face {
  font-family: 'Satoshi';
  src: url('/fonts/satoshi-700.woff2') format('woff2');
  font-style: normal;
  font-weight: 700;
  font-display: swap;
}

html.lumilio-landing-page {
  background: #f2efe8;
  scroll-behavior: smooth;
}

html.lumilio-landing-page body {
  background: #f2efe8;
}

.lumilio-landing,
.lumilio-landing * {
  box-sizing: border-box;
}

.lumilio-landing {
  --ink: #151512;
  --paper: #f2efe8;
  --paper-muted: #e7e2d9;
  --night: #11120f;
  --night-soft: #1a1b17;
  --muted: #5c5a54;
  --acid: #c8f169;
  --orange: #ff7a45;
  --blue: #9ebdff;
  min-width: 320px;
  overflow: clip;
  background: var(--paper);
  color: var(--ink);
  font-family: 'Satoshi', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  font-size: 16px;
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
}

.lumilio-landing a {
  color: inherit;
  text-decoration: none;
}

.lumilio-landing img {
  display: block;
  max-width: 100%;
}

.section-shell {
  width: min(100% - 48px, 1480px);
  margin-inline: auto;
}

.landing-nav {
  position: fixed;
  z-index: 100;
  top: 18px;
  left: 50%;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  width: min(calc(100% - 36px), 1460px);
  min-height: 66px;
  padding: 10px 12px 10px 16px;
  border: 1px solid rgba(21, 21, 18, 0.14);
  border-radius: 18px;
  background: rgba(242, 239, 232, 0.92);
  box-shadow: 0 18px 60px rgba(39, 37, 31, 0.12);
  backdrop-filter: blur(16px);
  transform: translateX(-50%);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  width: max-content;
  font-weight: 700;
  letter-spacing: -0.02em;
}

.brand img {
  width: 34px;
  height: 34px;
  object-fit: contain;
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 32px;
  color: #4d4b45;
  font-size: 14px;
  font-weight: 500;
}

.nav-links a,
.footer-links a,
.text-link,
.card-link {
  transition: color 180ms ease, opacity 180ms ease;
}

.nav-links a:hover,
.footer-links a:hover,
.text-link:hover,
.card-link:hover {
  color: #8a430f;
}

.nav-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  justify-self: end;
  min-height: 44px;
  padding: 0 18px;
  border-radius: 12px;
  background: var(--night);
  color: #f7f4ed !important;
  font-size: 14px;
  font-weight: 600;
}

.hero {
  display: grid;
  grid-template-columns: minmax(420px, 0.84fr) minmax(0, 1.16fr);
  gap: clamp(48px, 6vw, 110px);
  align-items: center;
  min-height: 100svh;
  padding-top: 136px;
  padding-bottom: 100px;
}

.hero-copy {
  position: relative;
  z-index: 2;
}

.eyebrow,
.section-kicker,
.capability-number,
.command-label {
  margin: 0 0 26px;
  color: #69665f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.hero h1 {
  max-width: 720px;
  margin: 0;
  font-size: clamp(64px, 6vw, 96px);
  font-weight: 500;
  line-height: 0.92;
  letter-spacing: -0.07em;
}

.hero-lede {
  max-width: 610px;
  margin: 38px 0 0;
  color: var(--muted);
  font-size: clamp(18px, 1.5vw, 23px);
  line-height: 1.65;
}

.hero-actions,
.cta-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
  margin-top: 36px;
}

.button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 11px;
  min-height: 54px;
  padding: 0 23px;
  border: 1px solid transparent;
  border-radius: 13px;
  font-size: 15px;
  font-weight: 700;
  transition: transform 180ms ease, background 180ms ease, color 180ms ease;
}

.button:hover,
.nav-action:hover {
  transform: translateY(-2px);
}

.button-primary {
  background: var(--acid);
  color: #12140e !important;
}

.text-link,
.card-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 22px;
  margin-top: 56px;
  color: #77736a;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.hero-meta span:not(:last-child)::after {
  content: '/';
  margin-left: 22px;
  color: #aaa59c;
}

.hero-stage {
  position: relative;
  width: 100%;
  padding: 12px;
  border: 1px solid rgba(21, 21, 18, 0.17);
  border-radius: 24px;
  background: #d8d1c5;
  box-shadow: 0 36px 100px rgba(37, 32, 23, 0.18);
  transform: rotate(1.2deg);
}

.hero-stage-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 0 10px;
  color: #4f4c46;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}

.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #739928;
  box-shadow: 0 0 0 4px rgba(115, 153, 40, 0.13);
}

.hero-stage-index {
  margin-left: auto;
}

.hero-screen {
  overflow: hidden;
  border: 1px solid rgba(21, 21, 18, 0.2);
  border-radius: 15px;
  background: #faf8f2;
}

.hero-screen img {
  width: 100%;
  aspect-ratio: 1.63;
  object-fit: cover;
}

.floating-photo {
  position: absolute;
  overflow: hidden;
  margin: 0;
  border: 6px solid #f3efe6;
  box-shadow: 0 25px 70px rgba(28, 24, 17, 0.25);
}

.floating-photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.floating-photo-left {
  right: 72%;
  bottom: -56px;
  width: clamp(130px, 13vw, 208px);
  aspect-ratio: 0.82;
  transform: rotate(-6deg);
}

.floating-photo-right {
  right: -42px;
  top: -72px;
  width: clamp(118px, 11vw, 174px);
  aspect-ratio: 0.72;
  transform: rotate(5deg);
}

.statement {
  padding-top: clamp(160px, 20vw, 310px);
  padding-bottom: clamp(150px, 19vw, 280px);
}

.section-heading {
  max-width: 1260px;
  margin: 0;
  font-size: clamp(48px, 7vw, 104px);
  font-weight: 500;
  line-height: 1.02;
  letter-spacing: -0.06em;
}

.inline-photo {
  display: inline-block;
  width: 1.45em;
  height: 0.72em;
  overflow: hidden;
  margin: 0 0.06em;
  border-radius: 999px;
  vertical-align: baseline;
  transform: rotate(-2deg);
}

.inline-photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.inline-photo-tall img {
  object-position: center 62%;
}

.statement-copy {
  max-width: 690px;
  margin: 64px 0 0 auto;
  color: var(--muted);
  font-size: clamp(19px, 2vw, 29px);
  line-height: 1.55;
}

.marquee {
  overflow: hidden;
  border-block: 1px solid rgba(21, 21, 18, 0.2);
  background: var(--orange);
}

.marquee-track {
  display: flex;
  align-items: center;
  width: max-content;
  min-height: 76px;
  animation: landing-marquee 28s linear infinite;
}

.marquee span {
  padding-inline: 28px;
  color: #1b0e08;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.marquee i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #1b0e08;
}

@keyframes landing-marquee {
  to { transform: translateX(-50%); }
}

.capability-stack {
  padding-top: 170px;
  padding-bottom: 220px;
}

.capability-card {
  position: sticky;
  top: 106px;
  display: grid;
  grid-template-columns: minmax(0, 0.82fr) minmax(0, 1.18fr);
  gap: clamp(44px, 6vw, 100px);
  min-height: 740px;
  margin-bottom: 150px;
  padding: clamp(38px, 5vw, 76px);
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 34px;
  color: #f2efe8;
  box-shadow: 0 45px 110px rgba(24, 22, 17, 0.24);
  transform-origin: center top;
}

.capability-agent {
  z-index: 1;
  background: #12130f;
}

.capability-lumen {
  z-index: 2;
  background: #172116;
}

.capability-search {
  z-index: 3;
  margin-bottom: 0;
  background: #101824;
}

.capability-copy {
  display: flex;
  align-items: flex-start;
  flex-direction: column;
}

.capability-card .capability-number,
.capability-card .command-label {
  color: #aaa89f;
}

.capability-card h2 {
  max-width: 680px;
  margin: 42px 0 28px;
  color: #f2efe8;
  font-size: clamp(42px, 5vw, 75px);
  font-weight: 500;
  line-height: 1.02;
  letter-spacing: -0.055em;
}

.capability-card .capability-copy > p {
  max-width: 610px;
  margin: 0;
  color: #b9b5ac;
  font-size: clamp(17px, 1.4vw, 21px);
  line-height: 1.72;
}

.mode-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 36px;
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
  padding-top: 45px;
  color: var(--acid);
}

.agent-proof,
.search-shot,
.lumen-visual {
  align-self: center;
}

.agent-proof {
  width: 100%;
  padding: 22px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 22px;
  background: #1c1d19;
}

.agent-command {
  padding: 22px;
  border-radius: 15px;
  background: #f1eee7;
  color: var(--ink);
}

.agent-command p {
  margin: 0;
  font-size: clamp(17px, 1.4vw, 21px);
  line-height: 1.6;
}

.tool-chain {
  display: grid;
  gap: 1px;
  margin: 18px 0;
  overflow: hidden;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.1);
}

.tool-chain div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 49px;
  padding: 0 17px;
  background: #23241f;
  color: #c9c6be;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.tool-chain b {
  color: var(--acid);
  font-weight: 500;
}

.confirm-shot {
  margin: 0;
  overflow: hidden;
  border-radius: 14px;
  background: #f3efe7;
}

.confirm-shot img {
  width: 100%;
}

.confirm-shot figcaption,
.search-shot figcaption {
  padding: 13px 16px;
  color: #77736a;
  font-size: 11px;
  line-height: 1.45;
}

.lumen-visual {
  position: relative;
  min-height: 620px;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 28px;
  background: #0f160e;
}

.node {
  position: absolute;
  z-index: 2;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 7px;
  width: 168px;
  min-height: 92px;
  padding: 18px;
  border: 1px solid rgba(200, 241, 105, 0.26);
  border-radius: 16px;
  background: #1c2819;
}

.node span {
  color: #889681;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.node b {
  color: #e2edcf;
  font-size: 14px;
  font-weight: 500;
}

.node-hub {
  left: 50%;
  top: 50%;
  align-items: center;
  width: 190px;
  min-height: 190px;
  border-radius: 999px;
  background: var(--acid);
  color: #12140e;
  text-align: center;
  transform: translate(-50%, -50%);
}

.node-hub img {
  width: 54px;
  height: 54px;
  object-fit: contain;
}

.node-hub span {
  color: #516222;
}

.node-one { left: 5%; top: 8%; }
.node-two { right: 5%; top: 8%; }
.node-three { left: 5%; bottom: 8%; }
.node-four { right: 5%; bottom: 8%; }

.node-lines {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
}

.node-lines path {
  fill: none;
  stroke: rgba(200, 241, 105, 0.32);
  stroke-dasharray: 5 7;
  stroke-width: 1.5;
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
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.14);
  border-radius: 24px;
  background: #eeeae2;
}

.search-shot img {
  width: 100%;
  min-height: 430px;
  object-fit: cover;
  object-position: center;
}

.search-shot figcaption {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #eeeae2;
  color: #252725;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
}

.search-shot figcaption span {
  color: #6a6e76;
  font-family: 'Satoshi', sans-serif;
}

.proof-section {
  padding-top: 110px;
  padding-bottom: 250px;
}

.proof-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 40px;
  margin-bottom: 72px;
}

.proof-header .section-heading {
  max-width: 900px;
  font-size: clamp(48px, 6.2vw, 90px);
}

.proof-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  color: #68655f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.09em;
}

.proof-controls button,
.proof-dots button {
  appearance: none;
  border: 0;
  cursor: pointer;
}

.proof-controls button {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border: 1px solid rgba(21, 21, 18, 0.22);
  border-radius: 999px;
  background: transparent;
  color: var(--ink);
}

.proof-frame {
  display: grid;
  grid-template-columns: minmax(290px, 0.36fr) minmax(0, 1fr);
  min-height: 680px;
  overflow: hidden;
  border-radius: 30px;
  background: #d8d2c7;
}

.proof-copy {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  padding: clamp(32px, 5vw, 70px);
}

.proof-copy > p {
  margin: 0 0 22px;
  color: #6b675f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.proof-copy h3 {
  margin: 0;
  font-size: clamp(32px, 3.5vw, 54px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.045em;
}

.proof-copy > span {
  margin-top: 28px;
  color: var(--muted);
  line-height: 1.7;
}

.proof-dots {
  display: flex;
  gap: 8px;
  margin-top: 54px;
}

.proof-dots button {
  width: 36px;
  height: 3px;
  background: #9e998f;
  transition: background 180ms ease, width 180ms ease;
}

.proof-dots button.active {
  width: 72px;
  background: var(--ink);
}

.proof-image-wrap {
  position: relative;
  min-height: 680px;
  overflow: hidden;
  background: #151512;
}

.proof-image-wrap img {
  width: 100%;
  height: 100%;
  min-height: 680px;
  object-fit: cover;
  object-position: center;
}

.proof-fade-enter-active,
.proof-fade-leave-active {
  transition: opacity 280ms ease, transform 320ms ease;
}

.proof-fade-enter-from,
.proof-fade-leave-to {
  opacity: 0;
  transform: scale(1.02);
}

.integrity-section {
  padding-bottom: 250px;
}

.integrity-section .section-heading {
  max-width: 1150px;
}

.integrity-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-flow: dense;
  gap: 18px;
  margin-top: 92px;
}

.integrity-card {
  position: relative;
  overflow: hidden;
  border-radius: 25px;
}

.integrity-primary {
  grid-column: span 7;
  grid-row: span 2;
  display: grid;
  min-height: 680px;
  padding: 46px;
  background: #151512;
  color: #f2efe8;
}

.integrity-secondary,
.integrity-tertiary {
  grid-column: span 5;
  min-height: 331px;
  padding: 40px;
}

.integrity-secondary { background: var(--orange); color: #1b0e08; }
.integrity-tertiary { background: #cbd8f3; color: #101824; }

.integrity-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 52px;
  border: 1px solid currentColor;
  border-radius: 999px;
}

.integrity-card p {
  margin: 0 0 16px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.integrity-card h3 {
  max-width: 630px;
  margin: 0;
  font-size: clamp(31px, 4vw, 60px);
  font-weight: 500;
  line-height: 1.04;
  letter-spacing: -0.05em;
}

.integrity-secondary h3,
.integrity-tertiary h3 {
  font-size: clamp(27px, 2.8vw, 43px);
}

.integrity-card span {
  display: block;
  max-width: 580px;
  margin-top: 22px;
  color: currentColor;
  opacity: 0.76;
  font-size: 15px;
  line-height: 1.65;
}

.integrity-primary > img {
  align-self: end;
  width: 100%;
  height: 270px;
  margin-top: 48px;
  border-radius: 16px;
  object-fit: cover;
}

.daily-section {
  display: grid;
  grid-template-columns: minmax(300px, 0.7fr) minmax(0, 1.3fr);
  gap: clamp(50px, 9vw, 160px);
  padding-bottom: 240px;
}

.daily-heading {
  position: sticky;
  top: 130px;
  align-self: start;
}

.daily-heading .section-heading {
  font-size: clamp(46px, 5vw, 72px);
}

.daily-list {
  border-top: 1px solid rgba(21, 21, 18, 0.22);
}

.daily-list article {
  display: grid;
  grid-template-columns: 54px 0.7fr 1fr 20px;
  gap: 24px;
  align-items: center;
  min-height: 118px;
  border-bottom: 1px solid rgba(21, 21, 18, 0.22);
}

.daily-list article > span {
  color: #89857c;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.daily-list h3,
.daily-list p {
  margin: 0;
}

.daily-list h3 {
  font-size: 23px;
  font-weight: 500;
  letter-spacing: -0.02em;
}

.daily-list p {
  color: var(--muted);
  font-size: 14px;
}

.local-section {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 90px;
  padding-top: clamp(90px, 10vw, 150px);
  padding-bottom: clamp(90px, 10vw, 150px);
  border-radius: 34px;
  background: var(--night);
  color: #f2efe8;
}

.local-copy {
  padding-left: clamp(34px, 7vw, 100px);
}

.local-copy .section-kicker { color: #9a968d; }

.local-copy .section-heading {
  max-width: 780px;
  color: #f2efe8;
  font-size: clamp(48px, 6vw, 86px);
}

.local-copy > p:last-child {
  max-width: 670px;
  margin: 46px 0 0;
  color: #b9b5ac;
  font-size: 18px;
  line-height: 1.8;
}

.local-stats {
  display: grid;
  align-content: center;
  padding-right: clamp(34px, 7vw, 100px);
}

.local-stats div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 24px;
  padding: 24px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.16);
}

.local-stats strong {
  color: var(--acid);
  font-size: clamp(54px, 7vw, 95px);
  font-weight: 500;
  line-height: 1;
}

.local-stats span {
  color: #b9b5ac;
  font-size: 13px;
}

.cta-section {
  display: grid;
  grid-template-columns: 0.86fr 1.14fr;
  min-height: 760px;
  margin-top: 180px;
  overflow: hidden;
  border-radius: 34px;
  background: var(--acid);
  color: #12140e;
}

.cta-visual {
  position: relative;
  min-height: 760px;
  overflow: hidden;
}

.cta-visual > img:first-child {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cta-bird {
  position: absolute;
  right: -40px;
  bottom: 40px;
  width: 42%;
  aspect-ratio: 1;
  border: 7px solid var(--acid);
  border-radius: 999px;
  object-fit: cover;
}

.cta-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: clamp(50px, 7vw, 110px);
}

.cta-copy .section-kicker { color: #506021; }

.cta-copy h2 {
  max-width: 800px;
  margin: 0;
  font-size: clamp(58px, 8vw, 118px);
  font-weight: 500;
  line-height: 0.94;
  letter-spacing: -0.065em;
}

.cta-copy > p:not(.section-kicker) {
  max-width: 610px;
  margin: 38px 0 0;
  color: #354217;
  font-size: 19px;
  line-height: 1.7;
}

.button-dark {
  background: #12140e;
  color: #f2efe8 !important;
}

.button-outline {
  border-color: rgba(18, 20, 14, 0.48);
  color: #12140e !important;
}

.landing-footer {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  align-items: center;
  min-height: 190px;
  margin-top: 70px;
  border-top: 1px solid rgba(21, 21, 18, 0.2);
}

.landing-footer p {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  text-align: center;
}

.footer-links {
  display: flex;
  justify-content: flex-end;
  gap: 28px;
  color: var(--muted);
  font-size: 14px;
}

@media (max-width: 1100px) {
  .landing-nav { grid-template-columns: 1fr auto; }
  .nav-links { display: none; }
  .hero { grid-template-columns: 1fr; min-height: auto; padding-top: 160px; }
  .hero-copy { max-width: 900px; }
  .hero-stage { width: calc(100% - 30px); margin: 60px 0 0 30px; }
  .floating-photo-left { right: auto; left: -40px; }
  .capability-card { grid-template-columns: 1fr; position: relative; top: auto; min-height: auto; }
  .capability-card .card-link { margin-top: 35px; padding-top: 0; }
  .lumen-visual { min-height: 560px; }
  .proof-frame { grid-template-columns: 1fr; }
  .proof-copy { min-height: 420px; }
  .daily-section { grid-template-columns: 1fr; }
  .daily-heading { position: relative; top: auto; }
  .local-section { width: min(100% - 36px, 1480px); grid-template-columns: 1fr; gap: 60px; }
  .local-copy { padding-right: clamp(34px, 7vw, 100px); }
  .local-stats { padding: 0 clamp(34px, 7vw, 100px); }
  .cta-section { grid-template-columns: 1fr; }
  .cta-visual { min-height: 520px; }
}

@media (max-width: 760px) {
  .section-shell { width: min(100% - 28px, 1480px); }
  .landing-nav { top: 10px; width: calc(100% - 20px); min-height: 58px; border-radius: 15px; }
  .brand span { font-size: 14px; }
  .brand img { width: 30px; height: 30px; }
  .nav-action { min-height: 40px; padding: 0 13px; font-size: 12px; }
  .hero { gap: 10px; padding-top: 120px; padding-bottom: 90px; }
  .hero h1 { font-size: clamp(58px, 18vw, 82px); }
  .hero-lede { margin-top: 28px; font-size: 17px; }
  .hero-actions { align-items: flex-start; flex-direction: column; }
  .hero-meta { gap: 8px 12px; margin-top: 38px; }
  .hero-meta span::after { display: none; }
  .hero-stage { width: 100%; margin: 55px 0 0; padding: 7px; border-radius: 18px; transform: none; }
  .hero-stage-bar { min-height: 36px; font-size: 9px; }
  .hero-stage-index { display: none; }
  .floating-photo-left { left: -8px; bottom: -60px; width: 110px; }
  .floating-photo-right { right: -10px; top: -52px; width: 98px; }
  .statement { padding-top: 170px; padding-bottom: 160px; }
  .section-heading { font-size: clamp(43px, 13vw, 68px); }
  .statement-copy { margin-top: 42px; font-size: 18px; }
  .marquee-track { min-height: 62px; }
  .capability-stack { padding-top: 100px; padding-bottom: 130px; }
  .capability-card { gap: 50px; margin-bottom: 70px; padding: 28px 20px; border-radius: 24px; }
  .capability-card h2 { margin-top: 32px; font-size: clamp(40px, 12vw, 58px); }
  .capability-card .capability-copy > p { font-size: 16px; }
  .agent-proof { padding: 12px; }
  .agent-command { padding: 17px; }
  .tool-chain div { padding: 0 12px; font-size: 10px; }
  .lumen-visual { min-height: 520px; }
  .node { width: 132px; min-height: 80px; padding: 13px; }
  .node-hub { width: 150px; min-height: 150px; }
  .node-one, .node-three { left: 3%; }
  .node-two, .node-four { right: 3%; }
  .node-lines { display: none; }
  .search-weights { gap: 14px; margin-top: 36px; padding-top: 0; }
  .search-shot img { min-height: 240px; }
  .proof-section { padding-top: 50px; padding-bottom: 160px; }
  .proof-header { align-items: flex-start; flex-direction: column; margin-bottom: 46px; }
  .proof-frame { min-height: 0; border-radius: 22px; }
  .proof-copy { min-height: 390px; padding: 28px; }
  .proof-image-wrap, .proof-image-wrap img { min-height: 300px; }
  .proof-image-wrap img { object-position: left center; }
  .integrity-section { padding-bottom: 160px; }
  .integrity-grid { grid-template-columns: 1fr; margin-top: 58px; }
  .integrity-primary, .integrity-secondary, .integrity-tertiary { grid-column: auto; grid-row: auto; min-height: auto; padding: 30px; }
  .integrity-primary { min-height: 610px; }
  .integrity-icon { margin-bottom: 40px; }
  .daily-section { gap: 55px; padding-bottom: 160px; }
  .daily-list article { grid-template-columns: 36px 1fr 18px; gap: 13px; padding: 22px 0; }
  .daily-list article p { grid-column: 2 / -1; }
  .local-section { gap: 45px; border-radius: 24px; }
  .local-copy, .local-stats { padding-inline: 27px; }
  .local-stats strong { font-size: 54px; }
  .cta-section { width: calc(100% - 28px); min-height: 0; margin-top: 110px; border-radius: 24px; }
  .cta-visual { min-height: 360px; }
  .cta-copy { padding: 52px 26px 58px; }
  .cta-copy h2 { font-size: clamp(56px, 17vw, 84px); }
  .cta-actions { align-items: stretch; flex-direction: column; }
  .cta-actions .button { width: 100%; }
  .landing-footer { grid-template-columns: 1fr; gap: 26px; padding: 48px 0; text-align: center; }
  .footer-brand { justify-self: center; }
  .footer-links { justify-content: center; }
}

@media (prefers-reduced-motion: reduce) {
  html.lumilio-landing-page { scroll-behavior: auto; }
  .marquee-track { animation: none; }
  .lumilio-landing *,
  .lumilio-landing *::before,
  .lumilio-landing *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
