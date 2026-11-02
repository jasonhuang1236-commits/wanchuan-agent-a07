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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { ErrorState } from '@/components/error-state'
import { getSelf } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import { REFERRAL_RECORDS_PAGE_SIZE } from '../constants'
import {
  useAffiliate,
  useReferralRewards,
  useTopupInfo,
  useWalletSummary,
} from '../hooks'
import type { UserWalletData } from '../types'
import { AffiliateRewardsCard } from './affiliate-rewards-card'
import { TransferDialog } from './dialogs/transfer-dialog'

export function ReferralPanel() {
  const client = useQueryClient()
  const [page, setPage] = useState(1)
  const [transferOpen, setTransferOpen] = useState(false)
  const rewards = useReferralRewards(page, REFERRAL_RECORDS_PAGE_SIZE)
  const affiliate = useAffiliate()
  const wallet = useWalletSummary()
  const info = useTopupInfo()
  const user = useQuery({
    queryKey: ['wallet', 'user'],
    queryFn: async () =>
      requireServerSuccess(await getSelf()).data as UserWalletData,
  })
  if (user.isError || wallet.error)
    {return (
      <ErrorState
        onRetry={() => {
          void user.refetch()
          void wallet.refetch()
        }}
      />
    )}
  return (
    <>
      <AffiliateRewardsCard
        user={user.data ?? null}
        affiliateLink={affiliate.affiliateLink}
        onTransfer={() => setTransferOpen(true)}
        availableQuota={wallet.summary?.referral_available_quota ?? 0}
        complianceConfirmed={
          info.topupInfo?.payment_compliance_confirmed === true
        }
        referralRate={wallet.summary?.rules.referral_bonus_rate ?? 0}
        records={rewards.records}
        recordsTotal={rewards.total}
        recordsLoading={rewards.loading}
        recordsError={rewards.error}
        onRecordsRetry={() => void rewards.refetch()}
        recordsPage={page}
        onRecordsPageChange={setPage}
        loading={user.isPending || wallet.loading || affiliate.loading}
      />
      <TransferDialog
        open={transferOpen}
        onOpenChange={setTransferOpen}
        availableQuota={wallet.summary?.referral_available_quota ?? 0}
        transferring={affiliate.transferring}
        onConfirm={async (quota) => {
          const success = await affiliate.transferQuota(quota)
          if (success) {
            await client.invalidateQueries({ queryKey: ['wallet'] })
            await client.invalidateQueries({ queryKey: ['agent-platform'] })
          }
          return success
        }}
      />
    </>
  )
}
