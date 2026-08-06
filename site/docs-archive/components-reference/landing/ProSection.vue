<script setup lang="ts">
const techStack = [
  ['GO', '高并发媒体处理管线', '基于libvips, ffmpeg 和 exiftool 的稳定媒体处理'],
  ['RUST', '高性能 AI 推理引擎', '基于 CUDA 和 WGPU 的跨平台计算后端'],
  ['量化', 'INT8量化模型，降低内存占用', '让本地推理在更多设备上可用，为复杂工作流腾出空间'],
]
</script>

<template>
  <section id="pro" class="pro-section section-shell">
    <p class="section-kicker">PRO · 为本地部署而生</p>
    <h2 class="section-heading">NAS 安静运行，<br />GPU 按需接管。</h2>
    <p class="pro-lede">
      流明集为本地部署与大规模媒体库设计。现代技术栈贯穿关键路径：
      高性能、低成本，从家用服务器到 GPU 工作站都能找到合适的姿势。
    </p>

    <div class="pro-grid">
      <article v-for="tech in techStack" :key="tech[0]">
        <div class="tech-label">
          <span v-if="tech[0] === 'GO'" class="tech-mark tech-mark-go" role="img" aria-label="Go">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 192" aria-hidden="true">
              <path d="M0 0h512v192H0z" fill="none" />
              <path fill="#00acd7" d="m292.533 13.295l1.124.75c13.212 8.725 22.685 20.691 28.917 35.15c1.496 2.243.499 3.49-2.493 4.237l-5.063 1.296c-11.447 2.949-20.53 5.429-31.827 8.378l-6.443 1.678c-2.32.574-2.96.333-5.428-2.477l-.348-.399c-3.519-3.988-6.155-6.652-10.817-9.03l-.899-.443c-15.705-7.727-30.911-5.484-45.12 3.74c-16.952 10.968-25.677 27.172-25.428 47.364c.25 19.942 13.96 36.395 33.654 39.137c16.951 2.244 31.16-3.739 42.378-16.452c2.244-2.743 4.238-5.734 6.73-9.224h-48.11c-5.235 0-6.481-3.24-4.736-7.478l.864-2.035c3.204-7.454 8.173-18.168 11.4-24.294l.704-1.319c.862-1.494 2.612-3.513 5.977-3.513h80.224c3.603-11.415 9.449-22.201 17.246-32.407c18.198-23.931 40.135-36.396 69.8-41.63c25.427-4.488 49.359-1.995 71.046 12.713c19.694 13.461 31.909 31.66 35.15 55.59c4.237 33.654-5.485 61.075-28.668 84.508c-16.453 16.702-36.645 27.172-59.829 31.908c-6.73 1.247-13.461 1.496-19.942 2.244c-22.685-.499-43.376-6.98-60.826-21.937c-12.273-10.61-20.727-23.648-24.928-38.828a105 105 0 0 1-10.47 16.89c-17.949 23.683-41.381 38.39-71.046 42.38c-24.43 3.24-47.115-1.497-67.058-16.454c-18.447-13.96-28.917-32.407-31.66-55.34c-3.24-27.173 4.737-51.603 21.19-73.041c17.7-23.184 41.132-37.891 69.8-43.126c22.999-4.16 45.037-1.595 64.936 11.464M411.12 49.017l-.798.178c-23.183 5.235-38.14 19.942-43.624 43.375c-4.488 19.444 4.985 39.138 22.934 47.115c13.71 5.983 27.421 5.235 40.633-1.496c19.694-10.22 30.413-26.175 31.66-47.613c-.25-3.24-.25-5.734-.749-8.227c-4.436-24.401-26.664-38.324-50.056-33.332M116.416 94.564c.997 0 1.496.748 1.496 1.745l-.499 5.983c0 .997-.997 1.745-1.745 1.745l-54.344-.249c-.997 0-1.246-.748-.748-1.496l3.49-6.232c.499-.748 1.496-1.496 2.493-1.496zM121.9 71.63c.997 0 1.496.748 1.247 1.496l-1.995 5.983c-.249.997-1.246 1.495-2.243 1.495l-117.912.25c-.997 0-1.246-.499-.748-1.247l5.235-6.73c.499-.748 1.745-1.247 2.742-1.247zm12.963-22.934c.997 0 1.246.748.748 1.496l-4.238 6.481c-.499.748-1.745 1.496-2.493 1.496l-90.24-.25c-.998 0-1.247-.498-.749-1.246l5.235-6.73c.499-.748 1.745-1.247 2.742-1.247z" />
            </svg>
          </span>
          <span v-else-if="tech[0] === 'RUST'" class="tech-mark tech-mark-rust" role="img" aria-label="Rust">
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" aria-hidden="true">
              <path d="M0 0h32v32H0z" fill="none" />
              <path fill="#ff7043" d="m30 12l-4-2V6h-4l-2-4l-4 2l-4-2l-2 4H6v4l-4 2l2 4l-2 4l4 2v4h4l2 4l4-2l4 2l2-4h4v-4l4-2l-2-4ZM6 16a9.9 9.9 0 0 1 .842-4H10v8H6.842A9.9 9.9 0 0 1 6 16m10 10a9.98 9.98 0 0 1-7.978-4H16v-2h-2v-2h4c.819.819.297 2.308 1.179 3.37a1.89 1.89 0 0 0 1.46.63h3.34A9.98 9.98 0 0 1 16 26m-2-12v-2h4a1 1 0 0 1 0 2Zm11.158 6H24a2.006 2.006 0 0 1-2-2a2 2 0 0 1-2-2a3 3 0 0 0 3-3q0-.08-.004-.161A3.115 3.115 0 0 0 19.83 10H8.022a9.986 9.986 0 0 1 17.136 10" />
            </svg>
          </span>
          <span v-else>{{ tech[0] }}</span>
        </div>
        <h3>{{ tech[1] }}</h3>
        <p>{{ tech[2] }}</p>
      </article>
    </div>
  </section>
</template>

<style>
.pro-section {
  margin-top: 180px;
  padding: clamp(50px, 7vw, 100px) clamp(34px, 7vw, 100px) clamp(60px, 7vw, 110px);
  border-radius: 34px;
  background: var(--night-soft);
  color: #f2efe8;
}

.pro-section .section-kicker {
  color: #9a968d;
}

.pro-section .section-heading {
  max-width: 1000px;
  color: #f2efe8;
  font-size: clamp(46px, 5.5vw, 84px);
}

.pro-lede {
  max-width: 690px;
  margin: 46px 0 0;
  color: #b9b5ac;
  font-size: 18px;
  line-height: 1.8;
}

.pro-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
  margin-top: 76px;
}

.pro-grid article {
  padding: 32px 30px 38px;
  border: 1px solid rgba(255, 255, 255, 0.13);
  border-radius: 22px;
  background: rgba(255, 255, 255, 0.03);
}

.tech-label {
  display: flex;
  align-items: center;
  min-height: 34px;
}

.tech-label > span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 34px;
  padding: 6px 11px;
  border: 1px solid rgba(200, 241, 105, 0.4);
  border-radius: 8px;
  color: var(--acid);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.tech-label .tech-mark {
  justify-content: flex-start;
  width: 74px;
  padding: 4px 0;
  border: 0;
}

.tech-mark svg {
  display: block;
}

.tech-mark-go svg {
  width: 52px;
  height: auto;
}

.tech-mark-rust svg {
  width: 30px;
  height: 30px;
}

.pro-grid h3 {
  margin: 26px 0 0;
  font-size: clamp(22px, 1.8vw, 28px);
  font-weight: 500;
  letter-spacing: -0.025em;
  line-height: 1.15;
}

.pro-grid p {
  margin: 14px 0 0;
  color: #b9b5ac;
  font-size: 15px;
  line-height: 1.65;
}

@media (max-width: 1100px) {
  .pro-section { width: min(100% - 36px, 1480px); }
  .pro-grid { grid-template-columns: 1fr; }
}

@media (max-width: 760px) {
  .pro-section { width: calc(100% - 28px); margin-top: 110px; padding: 48px 27px 54px; border-radius: 24px; }
  .pro-lede { margin-top: 34px; font-size: 16px; }
  .pro-grid { margin-top: 50px; }
}
</style>
