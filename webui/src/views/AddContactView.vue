<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from '@/components/Button.vue'
import Input from '@/components/Input.vue'
import Card from '@/components/Card.vue'
import { createContact } from '@/api/client'

const router = useRouter()

const form = ref({
  email: '',
  first_name: '',
  last_name: '',
  phone: '',
})
const formError = ref('')
const saving = ref(false)

async function handleCreate() {
  if (!form.value.email) {
    formError.value = 'Email is required.'
    return
  }
  saving.value = true
  try {
    await createContact(form.value)
    router.push('/contacts')
  } catch (e: any) {
    formError.value = e.response?.data?.error || 'Failed to create contact.'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-2xl font-bold">Add Contact</h1>
      <p class="text-muted-foreground">Create a new contact</p>
    </div>

    <Card title="Add Contact">
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <Input
            v-model="form.first_name"
            label="First Name"
            placeholder="John"
          />
          <Input
            v-model="form.last_name"
            label="Last Name"
            placeholder="Doe"
          />
        </div>
        <Input
          v-model="form.email"
          type="email"
          label="Email"
          placeholder="john@example.com"
          :error="formError && !form.email ? 'Email is required' : ''"
        />
        <Input
          v-model="form.phone"
          type="tel"
          label="Phone"
          placeholder="+1 (555) 000-0000"
        />
        <p v-if="formError" class="text-sm text-destructive">{{ formError }}</p>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <Button variant="outline" @click="router.push('/contacts')">Cancel</Button>
          <Button :loading="saving" @click="handleCreate">Create</Button>
        </div>
      </template>
    </Card>
  </div>
</template>