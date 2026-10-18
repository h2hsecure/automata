<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Trash2 } from 'lucide-vue-next'
import Button from '@/components/Button.vue'
import Card from '@/components/Card.vue'
import Input from '@/components/Input.vue'
import { listSettings, upsertSetting, deleteSetting } from '@/api/client'
import { settingGroups, knownSettingKeys } from './settingsCatalog'
import type { SettingField } from './settingsCatalog'

const values = ref<Record<string, string>>({})
const original = ref<Record<string, string>>({})
const loading = ref(false)
const saving = ref(false)
const saveError = ref('')
const saveSuccess = ref(false)

const unknownKeys = computed(() =>
  Object.keys(values.value).filter((k) => !knownSettingKeys.has(k)),
)

const dirtyKeys = computed(() =>
  Object.keys(values.value).filter((k) => values.value[k] !== original.value[k]),
)

const isDirty = computed(() => dirtyKeys.value.length > 0)

function fieldChecked(field: SettingField): boolean {
  return values.value[field.key] !== 'false'
}

function setFieldChecked(field: SettingField, checked: boolean) {
  values.value[field.key] = checked ? 'true' : 'false'
}

async function loadSettings() {
  loading.value = true
  try {
    const data = await listSettings()
    const next: Record<string, string> = {}
    for (const group of settingGroups) {
      for (const field of group.fields) {
        next[field.key] = data[field.key] ?? ''
      }
    }
    for (const [k, v] of Object.entries(data)) {
      if (!knownSettingKeys.has(k)) next[k] = v
    }
    values.value = next
    original.value = { ...next }
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  saving.value = true
  saveError.value = ''
  saveSuccess.value = false
  try {
    for (const key of dirtyKeys.value) {
      const next = values.value[key]
      if (next === '') {
        await deleteSetting(key)
      } else {
        await upsertSetting(key, next)
      }
    }
    original.value = { ...values.value }
    saveSuccess.value = true
  } catch (e: any) {
    saveError.value = e.response?.data?.error || 'Failed to save settings.'
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between">
      <p class="text-sm text-muted-foreground">
        Global settings applied across all websites. Empty fields fall back to
        the defaults in config.yaml.
      </p>
      <Button :disabled="!isDirty || saving" :loading="saving" @click="handleSave">
        Save Changes
      </Button>
    </div>

    <p v-if="saveError" class="text-sm text-destructive">{{ saveError }}</p>
    <p v-if="saveSuccess" class="text-sm text-green-600">Settings saved.</p>

    <div v-if="loading" class="py-8 text-center text-sm text-muted-foreground">
      Loading settings…
    </div>

    <template v-else>
      <Card
        v-for="group in settingGroups"
        :key="group.id"
        :title="group.title"
        :description="group.description"
      >
        <div class="grid gap-4 sm:grid-cols-2">
          <template v-for="field in group.fields" :key="field.key">
            <div v-if="field.type === 'boolean'" class="flex items-center gap-2">
              <input
                :id="field.key"
                type="checkbox"
                class="h-4 w-4 rounded border"
                :checked="fieldChecked(field)"
                @change="setFieldChecked(field, ($event.target as HTMLInputElement).checked)"
              />
              <label :for="field.key" class="text-sm">
                {{ field.label }}
                <span v-if="field.description" class="text-muted-foreground">
                  — {{ field.description }}
                </span>
              </label>
            </div>
            <Input
              v-else
              v-model="values[field.key]"
              :label="field.label"
              :type="field.type"
              :placeholder="field.placeholder"
            />
          </template>
        </div>
      </Card>

      <Card
        v-if="unknownKeys.length"
        title="Other"
        description="Settings not part of a known group."
      >
        <div class="space-y-3">
          <div v-for="key in unknownKeys" :key="key" class="flex items-center gap-2">
            <code class="w-56 shrink-0 truncate text-sm bg-muted px-1.5 py-0.5 rounded">
              {{ key }}
            </code>
            <Input v-model="values[key]" placeholder="Value" />
            <button
              class="p-1.5 rounded hover:bg-muted text-destructive"
              title="Delete"
              @click="values[key] = ''"
            >
              <Trash2 class="h-4 w-4" />
            </button>
          </div>
        </div>
      </Card>
    </template>
  </div>
</template>