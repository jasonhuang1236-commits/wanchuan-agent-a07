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
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { submitPaymentForm } from '@/lib/payment-form'

import { getSimplePaymentMethods, requestSimpleRecharge } from '../simple-api'
import { ActionForm, type ActionField } from './action-form'

export function SimpleRechargeForm(props: { onClose: () => void }) {
  const { t } = useTranslation()
  const info = useQuery({
    queryKey: ['agent-platform', 'simple-recharge-info'],
    queryFn: getSimplePaymentMethods,
  })

  if (info.isPending) return <LoadingState />
  if (info.isError || !info.data) {
    return (
      <Dialog
        open
        title={t('Recharge agent balance')}
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
      >
        <ErrorState onRetry={() => void info.refetch()} />
      </Dialog>
    )
  }

  const methods = info.data.map((method) => ({
    type: method.payment_method,
    name: method.payment_method,
  }))
  const fields: ActionField[] = [
    {
      name: 'amount',
      label: 'Agent balance to add ($)',
      kind: 'integer',
      min: 1,
      max: Number.MAX_SAFE_INTEGER,
      description:
        'The agent balance is credited at a 10% discount after payment succeeds.',
    },
    {
      name: 'payment_method',
      label: 'Payment method',
      kind: 'select',
      options: methods.map((method) => ({
        value: method.type,
        label: method.name,
      })),
    },
  ]

  if (!methods.length) {
    return (
      <Dialog
        open
        title={t('Recharge agent balance')}
        description={t('No compatible online payment method is configured.')}
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
      >
        <p className='text-muted-foreground text-sm'>
          {t('Contact the root administrator to configure a payment method.')}
        </p>
      </Dialog>
    )
  }

  return (
    <ActionForm
      title={t('Recharge agent balance')}
      description={t(
        'This payment is separate from your ordinary wallet. A successful payment adds the full agent balance shown above.'
      )}
      fields={fields}
      initial={{ payment_method: methods[0].type }}
      onClose={props.onClose}
      onSubmit={async (values) => {
        const result = await requestSimpleRecharge({
          amount: Number(values.amount),
          payment_method: String(values.payment_method),
        })
        submitPaymentForm(result.url, result.data)
        props.onClose()
      }}
      submitLabel={t('Continue to payment')}
    />
  )
}
