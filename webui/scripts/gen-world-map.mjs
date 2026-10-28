// One-off generator for src/assets/world-countries.json.
// Produces { ALPHA2: { name, d, cx, cy } } from world-atlas (110m) so the
// dashboard can render a dependency-free SVG world map.
//
// Usage: node scripts/gen-world-map.mjs
// Re-run only if the projection or world-atlas version changes; the JSON is
// committed to the repo.
import { readFileSync, writeFileSync } from 'node:fs'
import { geoNaturalEarth1, geoPath } from 'd3-geo'
import { feature } from 'topojson-client'
import iso3166 from 'iso-3166-1'

// numeric (zero-padded string) -> alpha2
const numericToAlpha2 = new Map(
  iso3166.all().map((c) => [String(Number(c.numeric)), c.alpha2]),
)
const toAlpha2 = (id) => numericToAlpha2.get(String(Number(id)))

const atlasPath = new URL('../node_modules/world-atlas/countries-110m.json', import.meta.url)

const topo = JSON.parse(readFileSync(atlasPath, 'utf8'))
const countries = feature(topo, topo.objects.countries)

const WIDTH = 960
const HEIGHT = 500
const projection = geoNaturalEarth1().fitSize([WIDTH, HEIGHT], countries)
const path = geoPath(projection)

// For countries with far-flung territories (France, US, Russia, ...), the
// centroid of the full geometry lands in the ocean. Use the centroid of the
// largest polygon so count badges sit on the main landmass.
function largestPart(geo) {
  const polys =
    geo.type === 'Polygon'
      ? [geo]
      : geo.coordinates.map((c) => ({ type: 'Polygon', coordinates: c }))
  polys.sort((a, b) => path.area(b) - path.area(a))
  return polys[0]
}

const out = {}
let skipped = []
for (const f of countries.features) {
  const alpha2 = toAlpha2(f.id)
  if (!alpha2) {
    skipped.push(`${f.id} (${f.properties?.name})`)
    continue
  }
  const geo = f.geometry
  if (!geo || (geo.type !== 'Polygon' && geo.type !== 'MultiPolygon')) {
    skipped.push(`${f.id} (${f.properties?.name}, ${geo?.type ?? 'no geometry'})`)
    continue
  }
  const [cx, cy] = path.centroid(largestPart(f.geometry))
  out[alpha2] = {
    name: f.properties?.name ?? alpha2,
    d: path(f),
    cx: Number(cx.toFixed(1)),
    cy: Number(cy.toFixed(1)),
  }
}

const target = new URL('../src/assets/world-countries.json', import.meta.url)
writeFileSync(target, JSON.stringify(out))
console.log(`wrote ${target} with ${Object.keys(out).length} countries`)
if (skipped.length) console.log(`skipped (no ISO alpha-2): ${skipped.join(', ')}`)