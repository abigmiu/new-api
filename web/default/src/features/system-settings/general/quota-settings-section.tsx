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
import { useMemo, type ChangeEvent } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Input } from '@/components/design-system/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

function createQuotaSchema(percentageErrorMessage: string) {
  return z
    .object({
      QuotaForNewUser: z.coerce.number().min(0),
      PreConsumedQuota: z.coerce.number().min(0),
      QuotaForInviter: z.coerce.number().min(0),
      QuotaForInvitee: z.coerce.number().min(0),
      InviterRewardType: z.enum(['', 'fixed', 'percentage']),
      InviterRewardValue: z.coerce.number().int().min(0).max(2147483647),
      TopUpLink: z.string(),
      general_setting: z.object({
        docs_link: z.string(),
      }),
      quota_setting: z.object({
        enable_free_model_pre_consume: z.boolean(),
      }),
    })
    .superRefine((values, context) => {
      if (
        values.InviterRewardType === 'percentage' &&
        values.InviterRewardValue > 100
      ) {
        context.addIssue({
          code: z.ZodIssueCode.custom,
          message: percentageErrorMessage,
          path: ['InviterRewardValue'],
        })
      }
    })
}

type QuotaFormValues = z.infer<ReturnType<typeof createQuotaSchema>>
type QuotaInputValue = number | ''

type InviterRewardConfig = {
  type?: unknown
  value?: unknown
}

function parseInviterRewardConfig(value: string): {
  type: '' | 'fixed' | 'percentage'
  value: number
} {
  try {
    const config = JSON.parse(value) as InviterRewardConfig
    const type =
      config.type === 'fixed' || config.type === 'percentage' ? config.type : ''
    const rewardValue =
      typeof config.value === 'number' && Number.isInteger(config.value)
        ? Math.max(0, config.value)
        : 0
    return { type, value: rewardValue }
  } catch {
    return { type: '', value: 0 }
  }
}

function formatQuotaInputValue(value: QuotaInputValue): string {
  return formatQuota(value === '' ? 0 : value)
}

type QuotaSettingsSectionProps = {
  defaultValues: Omit<
    QuotaFormValues,
    'InviterRewardType' | 'InviterRewardValue'
  > & {
    InviterRewardConfig: string
  }
  complianceConfirmed?: boolean
}

export function QuotaSettingsSection({
  defaultValues,
  complianceConfirmed = true,
}: QuotaSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const quotaSchema = useMemo(
    () => createQuotaSchema(t('Percentage must be between 0 and 100.')),
    [t]
  )
  const inviterRewardConfig = parseInviterRewardConfig(
    defaultValues.InviterRewardConfig
  )
  const handleNumberChange =
    (onChange: (value: QuotaInputValue) => void) =>
    (event: ChangeEvent<HTMLInputElement>) => {
      const value = event.currentTarget.valueAsNumber
      onChange(Number.isNaN(value) ? '' : value)
    }

  const { form, handleSubmit, isDirty, isSubmitting } =
    useSettingsForm<QuotaFormValues>({
      resolver: zodResolver(quotaSchema) as Resolver<
        QuotaFormValues,
        unknown,
        QuotaFormValues
      >,
      defaultValues: {
        ...defaultValues,
        InviterRewardType: inviterRewardConfig.type,
        InviterRewardValue: inviterRewardConfig.value,
      },
      onSubmit: async (_data, changedFields) => {
        const changedRewardConfig =
          'InviterRewardType' in changedFields ||
          'InviterRewardValue' in changedFields
        if (changedRewardConfig) {
          const type = _data.InviterRewardType
          const value = _data.InviterRewardValue
          await updateOption.mutateAsync({
            key: 'InviterRewardConfig',
            value: JSON.stringify({ type, value }),
          })
        }
        for (const [key, value] of Object.entries(changedFields)) {
          if (key === 'InviterRewardType' || key === 'InviterRewardValue') {
            continue
          }
          await updateOption.mutateAsync({
            key,
            value: value as string | number | boolean,
          })
        }
      },
    })

  return (
    <SettingsSection title={t('Quota Settings')}>
      <FormNavigationGuard when={isDirty} />

      {!complianceConfirmed ? (
        <Alert variant='destructive'>
          <AlertDescription>
            {t(
              'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
            )}
          </AlertDescription>
        </Alert>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={handleSubmit}>
          <SettingsPageFormActions
            onSave={handleSubmit}
            isSaving={updateOption.isPending || isSubmitting}
          />
          <FormDirtyIndicator isDirty={isDirty} />
          <SettingsFormGrid>
            <FormField
              control={form.control}
              name='QuotaForNewUser'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('New User Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Initial quota given to new users ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='PreConsumedQuota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Pre-Consumed Quota')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota consumed before charging users')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInviter'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Inviter Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Quota given to users who invite others ({{formattedQuota}})',
                      {
                        formattedQuota: formatQuotaInputValue(field.value),
                      }
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='QuotaForInvitee'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitee Reward')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Quota given to invited users ({{formattedQuota}})', {
                      formattedQuota: formatQuotaInputValue(field.value),
                    })}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='InviterRewardType'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invite Recharge Rebate')}</FormLabel>
                  <Select
                    items={[
                      { value: '', label: t('Disabled') },
                      { value: 'fixed', label: t('Fixed quota') },
                      { value: 'percentage', label: t('Percentage') },
                    ]}
                    value={field.value}
                    onValueChange={(value) => {
                      if (
                        value === '' ||
                        value === 'fixed' ||
                        value === 'percentage'
                      ) {
                        field.onChange(value)
                      }
                    }}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent alignItemWithTrigger={false}>
                      <SelectGroup>
                        <SelectItem value=''>{t('Disabled')}</SelectItem>
                        <SelectItem value='fixed'>
                          {t('Fixed quota')}
                        </SelectItem>
                        <SelectItem value='percentage'>
                          {t('Percentage')}
                        </SelectItem>
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Reward inviters when referred users recharge.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='InviterRewardValue'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invite Recharge Rebate Value')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={0}
                      max={
                        form.watch('InviterRewardType') === 'percentage'
                          ? 100
                          : 2147483647
                      }
                      value={field.value ?? ''}
                      onChange={handleNumberChange(field.onChange)}
                      name={field.name}
                      onBlur={field.onBlur}
                      ref={field.ref}
                      disabled={form.watch('InviterRewardType') === ''}
                    />
                  </FormControl>
                  <FormDescription>
                    {form.watch('InviterRewardType') === 'percentage'
                      ? t(
                          'Percentage of the referred user recharge, from 0 to 100.'
                        )
                      : t(
                          'Fixed quota granted for each referred user recharge.'
                        )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <SettingsFormGridItem span='full'>
              <FormField
                control={form.control}
                name='quota_setting.enable_free_model_pre_consume'
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{t('Pre-Consume for Free Models')}</FormLabel>
                      <FormDescription>
                        {t(
                          'When enabled, zero-cost models also pre-consume quota before final settlement.'
                        )}
                      </FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                        disabled={updateOption.isPending}
                      />
                    </FormControl>
                  </SettingsSwitchItem>
                )}
              />
            </SettingsFormGridItem>

            <FormField
              control={form.control}
              name='TopUpLink'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Top-Up Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/topup')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('External link for users to purchase quota')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='general_setting.docs_link'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Documentation Link')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://docs.example.com')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Link to your documentation site')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </SettingsFormGrid>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
