<script setup lang="ts">
import { computed } from 'vue'
import world from '@/assets/world-countries.json'
import { colorForCount, type CountryStat } from '@/lib/worldMap'

const props = defineProps<{
  countries: CountryStat[]
}>()

const worldCountries = world as Record<string, { name: string; d: string; cx: number; cy: number }>

const byCode = computed(() => new Map(props.countries.map((c) => [c.code, c])))
const maxCount = computed(() => Math.max(1, ...props.countries.map((c) => c.count)))

function fillOf(code: string): string {
  const stat = byCode.value.get(code)
  return stat ? colorForCount(stat.count, maxCount.value) : '#e2e8f0'
}

function tooltipOf(code: string): string {
  const stat = byCode.value.get(code)
  if (!stat) return worldCountries[code].name
  const cities = stat.cities.length ? ` — ${stat.cities.join(', ')}` : ''
  return `${worldCountries[code].name}: ${stat.count} contact${stat.count === 1 ? '' : 's'}${cities}`
}
</script>

<template>
  <svg viewBox="0 0 960 500" class="h-auto w-full" role="img" aria-label="World map of contact locations">
    <g>
      <path
        v-for="(c, code) in worldCountries"
        :key="code"
        :d="c.d"
        :fill="fillOf(code)"
        :data-code="code"
        class="stroke-background transition-[fill] duration-200 hover:opacity-80 dark:stroke-white/10"
        stroke-width="0.5"
      >
        <title>{{ tooltipOf(code) }}</title>
      </path>
    </g>
    <template v-for="stat in countries" :key="stat.code">
      <g v-if="worldCountries[stat.code]" class="map-badge">
        <circle
          class="map-badge-dot"
          :cx="worldCountries[stat.code].cx"
          :cy="worldCountries[stat.code].cy"
          r="9"
          fill="white"
          stroke="#0f172a"
          stroke-width="1"
        />
        <text
          :x="worldCountries[stat.code].cx"
          :y="worldCountries[stat.code].cy"
          text-anchor="middle"
          dominant-baseline="central"
          class="fill-slate-900 text-[10px] font-bold"
        >
          {{ stat.count }}
        </text>
      </g>
    </template>
  </svg>
</template>