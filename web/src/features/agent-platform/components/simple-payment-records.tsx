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

import { CopyButton } from '@/components/copy-button'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'

import { formatAgentQuota as formatQuota } from '../lib/money'
import { getSimplePaymentOrders, type SimplePaymentOrder } from '../simple-api'

export function SimplePaymentRecords(props: { agentUserId?: number }) {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['agent-platform', 'simple-payment-orders', props.agentUserId],
    queryFn: () =>
      getSimplePaymentOrders({
        agent_user_id: props.agentUserId,
        p: 1,
        page_size: 50,
      }),
  })
  if (query.isPending) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  if (!query.data?.items.length) {
    return (
      <EmptyState
        title={t('No customer payment records')}
        description={t('Customer payment links will appear here.')}
      />
    )
  }
  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between'>
        <h2 className='text-sm font-semibold'>
          {t('Customer payment records')}
        </h2>
        <span className='text-muted-foreground text-xs'>
          {t('{{count}} records', { count: query.data.total })}
        </span>
      </div>
      <div className='divide-border overflow-hidden rounded-lg border'>
        {query.data.items.map((row) => (
          <SimplePaymentRow key={row.id} row={row} />
        ))}
      </div>
    </div>
  )
}

function SimplePaymentRow(props: { row: SimplePaymentOrder }) {
  const { t } = useTranslation()
  const status =
    {
      pending: t('Awaiting payment'),
      paid: t('Paid'),
      redeemed: t('Redeemed'),
      expired: t('Expired'),
      refunded: t('Refunded'),
    }[props.row.status] ?? props.row.status
  return (
    <div className='flex flex-wrap items-center gap-3 border-b p-3 last:border-b-0'>
      <div className='min-w-44 flex-1'>
        <div className='font-medium'>{props.row.name}</div>
        <div className='text-muted-foreground text-xs'>{status}</div>
      </div>
      <div className='text-right text-sm'>
        <div className='font-medium tabular-nums'>
          {formatQuota(props.row.quota)}
        </div>
        <div className='text-muted-foreground text-xs'>
          ¥{(props.row.sale_price_fen / 100).toFixed(2)}
        </div>
      </div>
      {props.row.customer_url && (
        <div className='flex min-w-56 items-center gap-1'>
          <code className='min-w-0 flex-1 truncate text-xs'>
            {props.row.customer_url}
          </code>
          <CopyButton
            value={props.row.customer_url}
            aria-label={t('Copy payment link')}
          />
        </div>
      )}
    </div>
  )
}
