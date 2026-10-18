// Catalog of known global settings, grouped by configuration area.
// The Settings tab renders these groups as forms; keys present in the DB
// but not listed here fall into the "Other" group in the UI.
//
// A field's stored value is a plain string. Empty means "not set" and the
// runtime falls back to the config.yaml default (see smtp.MergeConfig).

export type SettingFieldType = 'text' | 'password' | 'number' | 'boolean'

export interface SettingField {
  key: string
  label: string
  type: SettingFieldType
  placeholder?: string
  description?: string
  // Value assumed when the field is empty (display only).
  defaultValue?: string
}

export interface SettingGroup {
  id: string
  title: string
  description: string
  fields: SettingField[]
}

export const settingGroups: SettingGroup[] = [
  {
    id: 'smtp',
    title: 'Email (SMTP)',
    description:
      'Relay overrides for outgoing email. Empty fields fall back to the values in config.yaml.',
    fields: [
      { key: 'smtp.host', label: 'Host', type: 'text', placeholder: 'smtp.example.com' },
      { key: 'smtp.port', label: 'Port', type: 'number', placeholder: '587' },
      { key: 'smtp.username', label: 'Username', type: 'text', placeholder: 'username' },
      { key: 'smtp.password', label: 'Password', type: 'password', placeholder: '••••••••' },
      { key: 'smtp.from', label: 'From address', type: 'text', placeholder: 'noreply@example.com' },
      {
        key: 'smtp.starttls',
        label: 'STARTTLS',
        type: 'boolean',
        defaultValue: 'true',
        description: 'Enabled by default; uncheck to disable.',
      },
    ],
  },
]

export const knownSettingKeys: ReadonlySet<string> = new Set(
  settingGroups.flatMap((g) => g.fields.map((f) => f.key)),
)