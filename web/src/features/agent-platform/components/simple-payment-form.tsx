/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useRequestId } from '@/features/agent-platform/hooks/use-request-id'

import { parseAgentQuota } from '../lib/money'
import { createSimplePayment, getSimplePaymentMethods } from '../simple-api'
import { ActionForm, type ActionField } from './action-form'

function parseSalePrice(value: string) {
  if (!/^(0|[1-9]\d*)(\.\d{1,2})?$/.test(value)) return 0
  const [yuan, fen = ''] = value.split('.')
  const result = Number(yuan) * 100 + Number(fen.padEnd(2, '0'))
  return Number.isSafeInteger(result) ? result : 0
}

export function SimplePaymentForm(props: { onClose: () => void }) {
  const { t } = useTranslation()
  const requestId = useRequestId()
  const [createdURL, setCreatedURL] = useState('')
  const info = useQuery({
    queryKey: ['agent-platform', 'simple-payment-methods'],
    queryFn: () => getSimplePaymentMethods(),
  })

  if (info.isPending) return null
  const methods = info.data ?? []
  if (createdURL) {
    return (
      <Dialog
        open
        title={t('Customer payment link created')}
        description={t(
          'Send this link to the customer. The code is delivered after payment succeeds.'
        )}
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
        footer={<Button onClick={props.onClose}>{t('Done')}</Button>}
      >
        <div className='flex min-w-0 items-center gap-2 rounded-md border p-3'>
          <code className='min-w-0 flex-1 text-xs break-all'>{createdURL}</code>
          <CopyButton value={createdURL} aria-label={t('Copy payment link')} />
        </div>
      </Dialog>
    )
  }
  if (info.isError || !methods.length) {
    return (
      <Dialog
        open
        title={t('Generate customer payment code')}
        onOpenChange={(open) => !open && props.onClose()}
      >
        <p className='text-muted-foreground text-sm'>
          {t('No compatible online payment method is configured.')}
        </p>
      </Dialog>
    )
  }
  const fields: ActionField[] = [
    { name: 'name', label: 'Name', kind: 'text', max: 80 },
    { name: 'quota', label: 'Quota', kind: 'integer', min: 1 },
    {
      name: 'sale_price',
      label: 'Customer price (yuan)',
      kind: 'text',
      max: 16,
      validate: (value) =>
        typeof value === 'string' && parseSalePrice(value) > 0
          ? undefined
          : t('Enter a valid price with up to two decimal places'),
    },
    {
      name: 'payment_method',
      label: 'Payment method',
      kind: 'select',
      options: methods.map((method) => ({
        value: method.payment_method,
        label: method.payment_method,
      })),
    },
  ]
  return (
    <ActionForm
      title={t('Generate customer payment code')}
      description={t(
        'The payment code does not consume your agent balance. Your profit is frozen for five days after payment.'
      )}
      fields={fields}
      initial={{ payment_method: methods[0].payment_method }}
      onClose={props.onClose}
      onSubmit={async (values) => {
        const quota = parseAgentQuota(String(values.quota)) ?? 0
        const salePriceFen = parseSalePrice(String(values.sale_price))
        if (!Number.isSafeInteger(quota) || quota <= 0 || salePriceFen <= 0) {
          throw new Error(t('Invalid payment code request'))
        }
        const result = await createSimplePayment({
          request_id: requestId(values),
          name: String(values.name).trim(),
          quota,
          sale_price_fen: salePriceFen,
          payment_method: String(values.payment_method),
        })
        if (!result.order.customer_url) {
          throw new Error(t('Payment link was not created'))
        }
        setCreatedURL(result.order.customer_url)
        toast.success(t('Customer payment link created'))
      }}
      submitLabel={t('Create payment link')}
    />
  )
}
