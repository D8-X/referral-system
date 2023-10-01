# referral-system
Referral system

# API

## ok: Earned rebate
`earned-rebate?referrerAddr=0x...|traderAddr=0x...|agencyAddr=0x...`

## referral-volume
Calculate the total referred volume
`https://referral.zkdev.d8x.xyz/referral-volume?referrerAddr=0x20ec1a4332140f26d7b910554e3baaa429ca3756`
old:
 // 1) identify codes for the referrer
 // 2) for each code look-up code usage for (trader, from, to)
 // 3) query trading volume (trader, from, to) from API for each record found in 2
 // 4) aggregate
new:
1) identify all agencies/referrers in the referral_chain that are a child of the given agency
2) identify all codes that are issued by these addresses
3) continue as the old query

## ok: open-trader-rebate
## ok: agency-rebate
Now same as referral-rebate

## ok: referral-rebate
Queries how much rebate the referrer gets for the given token holding amount or with their agency
-> referral not in referral_chain, based on token holdings
-> referral as child in token_chain, find queue

## ok: my-referral-codes

## select-referral-code

## new: refer
as agency or broker (child in referral-chain or broker) we can add a new child.
the child (refer to address) is not allowed to be already in the quueue
This needs to be signed by the agency/broker and we need to check the signature


## upsert-referral-code
Anyone can create a code. The parent is either a child in the referral_chain, or
the broker

## Contracts
Generate the ABI:
`abigen --abi src/contracts/abi/MultiPay.json --pkg contracts --type MultiPay --out multi_pay.go`
