import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { SimpleAgentFinance } from '../simple-finance'

test('root can record an ordinary user withdrawal even when there are no agent accounts', async () => {
  useSystemConfigStore.getState().setConfig({
    currency: {
      displayInCurrency: true,
      customCurrencySymbol: '$',
      customCurrencyExchangeRate: 1,
      quotaDisplayType: 'USD',
      quotaPerUnit: 500_000,
      usdExchangeRate: 1,
    },
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: { items: [], total: 0, page: 1, page_size: 100 },
    },
  })
  const request = vi
    .spyOn(api, 'request')
    .mockResolvedValue({ data: { success: true, data: {} } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <SimpleAgentFinance />
    </QueryClientProvider>
  )
  fireEvent.click(
    await screen.findByRole('button', { name: 'Record user withdrawal' })
  )
  for (const [name, value] of [
    ['User ID', '73'],
    ['Referral reward ($)', '20.50'],
    ['Payment-code profit ($)', '0'],
    ['Reason', 'Synthetic support payout'],
    ['Support evidence', 'Synthetic payment reference'],
  ]) {
    fireEvent.change(screen.getByRole('textbox', { name }), {
      target: { value },
    })
  }
  fireEvent.click(
    screen.getByRole('checkbox', {
      name: 'I confirm this change and its financial impact',
    })
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(request).toHaveBeenCalledOnce())
  expect(request.mock.calls[0][0]).toMatchObject({
    url: '/api/agent-platform/simple/finance/withdrawal',
    data: { user_id: 73, referral_quota: 10_250_000, payment_quota: 0 },
  })
})
