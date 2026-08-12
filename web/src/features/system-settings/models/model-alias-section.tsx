/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import { ModelMappingEditor } from '@/features/channels/components/model-mapping-editor'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const aliasMappingSchema = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  try {
    const parsed = JSON.parse(trimmed)
    if (
      typeof parsed !== 'object' ||
      parsed === null ||
      Array.isArray(parsed)
    ) {
      return false
    }
    const mapping = parsed as Record<string, unknown>
    for (const [alias, target] of Object.entries(mapping)) {
      if (typeof target !== 'string' || !alias.trim() || !target.trim()) {
        return false
      }
      if (alias === target) {
        return false
      }
    }
    // 链式循环检测
    for (const alias of Object.keys(mapping)) {
      const visited = new Set([alias])
      let current = alias
      for (;;) {
        const next = mapping[current]
        if (typeof next !== 'string' || !next || next === current) break
        if (visited.has(next)) return false
        visited.add(next)
        current = next
      }
    }
    return true
  } catch {
    return false
  }
}, 'Alias mapping must be a JSON object of non-empty strings without self-mapping or cycles')

const schema = z.object({
  'model_alias_setting.mapping': aliasMappingSchema,
})

type ModelAliasFormValues = z.infer<typeof schema>

type ModelAliasSectionProps = {
  defaultValues: ModelAliasFormValues
}

function normalizeJsonText(value: string, fallback: string) {
  const trimmed = (value ?? '').toString().trim()
  return trimmed ? trimmed : fallback
}

export function ModelAliasSection({ defaultValues }: ModelAliasSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<ModelAliasFormValues>({
    resolver: zodResolver(schema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: ModelAliasFormValues) => {
    const value = normalizeJsonText(values['model_alias_setting.mapping'], '{}')
    if (
      value === normalizeJsonText(defaultValues['model_alias_setting.mapping'], '{}')
    ) {
      toast.info(t('No changes to save'))
      return
    }
    await updateOption.mutateAsync({
      key: 'model_alias_setting.mapping',
      value,
    })
  }

  return (
    <SettingsSection title={t('Global Model Alias')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='model_alias_setting.mapping'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Alias Mapping')}</FormLabel>
                <FormControl>
                  <ModelMappingEditor
                    value={field.value}
                    onChange={(value) => field.onChange(value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Aliases are resolved to the real model before channel selection, so routing, billing and logs all use the real model. Applies to all channels.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
