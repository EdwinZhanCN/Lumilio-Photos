<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted } from 'vue'
import LandingNav from './landing/LandingNav.vue'
import HeroSection from './landing/HeroSection.vue'
import StatementSection from './landing/StatementSection.vue'
import MarqueeSection from './landing/MarqueeSection.vue'
import CapabilityStack from './landing/CapabilityStack.vue'
import UnderstandSection from './landing/UnderstandSection.vue'
import ProofSection from './landing/ProofSection.vue'
import LibrarySection from './landing/LibrarySection.vue'
import IntegritySection from './landing/IntegritySection.vue'
import DailySection from './landing/DailySection.vue'
import LocalSection from './landing/LocalSection.vue'
import ProSection from './landing/ProSection.vue'
import CtaSection from './landing/CtaSection.vue'
import LandingFooter from './landing/LandingFooter.vue'
import './landing/base.css'

let animationContext: { revert: () => void } | undefined
let capabilityMedia: { revert: () => void } | undefined
let refreshFrame = 0

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
      // 以承载卡片为滚动锚点：图片在卡片内不一定居中（如贴底的地图截图），
      // 直接用图片自身做 trigger 会出现“卡片已在屏幕中央、图片仍未显影”。
      const trigger =
        image.closest<HTMLElement>(
          '.understand-card, .library-card, .integrity-card, .cta-visual, .hero-screen',
        ) ?? image
      gsap.fromTo(
        image,
        { opacity: 0, scale: 1.08 },
        {
          opacity: 1,
          scale: 1,
          ease: 'none',
          scrollTrigger: {
            trigger,
            start: 'top 88%',
            end: 'top 42%',
            scrub: 0.8,
          },
        },
      )
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

  capabilityMedia = gsap.matchMedia()
  capabilityMedia.add('(min-width: 1101px) and (min-height: 701px)', () => {
    const cards = gsap.utils.toArray<HTMLElement>('.capability-card')
    cards.slice(0, -1).forEach((card, index) => {
      const shade = card.querySelector<HTMLElement>('.capability-shade')
      const timeline = gsap.timeline({
        scrollTrigger: {
          trigger: cards[index + 1],
          start: 'top 82%',
          end: 'top 24%',
          scrub: true,
        },
      })
      timeline.to(card, { scale: 0.94 - index * 0.01, y: -26, ease: 'none' }, 0)
      if (shade) timeline.to(shade, { opacity: 0.52, ease: 'none' }, 0)
    })
  })

  refreshFrame = window.requestAnimationFrame(() => ScrollTrigger.refresh())
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(refreshFrame)
  capabilityMedia?.revert()
  animationContext?.revert()
  document.documentElement.classList.remove('lumilio-landing-page')
})
</script>

<template>
  <div class="lumilio-landing">
    <LandingNav />

    <main>
      <HeroSection />
      <StatementSection />
      <MarqueeSection />
      <LibrarySection />
      <UnderstandSection />
      <CapabilityStack />
      <ProofSection />
      <IntegritySection />
      <DailySection />
      <LocalSection />
      <ProSection />
      <CtaSection />
    </main>

    <LandingFooter />
  </div>
</template>
