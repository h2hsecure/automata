<script setup lang="ts">
import { computed } from 'vue'

interface Column {
  key: string
  label: string
  sortable?: boolean
  width?: string
}

const props = defineProps<{
  columns: Column[]
  data: any[]
  loading?: boolean
  emptyText?: string
  onSort?: (key: string) => void
  onRowClick?: (row: any) => void
}>()

const sortKey = computed(() => '')
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full text-sm">
      <thead>
        <tr class="border-b bg-muted/50">
          <th
            v-for="col in columns"
            :key="col.key"
            :class="[
              'px-4 py-3 text-left font-medium text-muted-foreground',
              col.sortable ? 'cursor-pointer hover:text-foreground' : '',
              col.width ? `w-[${col.width}]` : '',
            ]"
            @click="col.sortable && props.onSort?.(col.key)"
          >
            {{ col.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-if="loading"
          class="border-b"
        >
          <td :colspan="columns.length" class="px-4 py-8 text-center text-muted-foreground">
            Loading...
          </td>
        </tr>
        <tr
          v-else-if="data.length === 0"
          class="border-b"
        >
          <td :colspan="columns.length" class="px-4 py-8 text-center text-muted-foreground">
            {{ emptyText || 'No results found.' }}
          </td>
        </tr>
        <tr
          v-for="(row, index) in data"
          :key="index"
          :class="[
            'border-b transition-colors',
            props.onRowClick ? 'cursor-pointer hover:bg-muted/50' : '',
          ]"
          @click="props.onRowClick?.(row)"
        >
          <td
            v-for="col in columns"
            :key="col.key"
            class="px-4 py-3"
          >
            <slot :name="col.key" :row="row">
              {{ row[col.key] ?? '-' }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
