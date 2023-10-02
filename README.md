# referral-system
Referral system

# Todos

- walk back in time block-time

# API

## Select a referral code as a trader

http://127.0.0.1:8000/select-code

```
 {
      "code": "THUANBX",
      "traderAddr": "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
      "createdOn": 1696264745,
      "signature": "0x4712cd2d1b772442afe1ea7f4c2a1f0cb8cc1bbc320a9bb449edd29f71e20ddd25b81daeb5a026f0d5e8361fe3739a365590ca9a6f416807ece8dd0ee3fd0a0e1b"
}
```
Success:
```
{
    "type": "select-code",
    "data": {
        "code": DOUBLE_AG
    }
}
```
Error:
`{"error":"code selection failed:Code already selected"}`
`{"error":"code selection failed:Failed"}``

## Update or create a code (anyone can be referrer)

http://127.0.0.1:8000/upsert-code

```
{
      "code": "ABCD",
      "referrerAddr": "0x0aB6527027EcFF1144dEc3d78154fce309ac838c",
      "createdOn": 1696166434,
      "PassOnPercTDF": 225,
      "signature": "0xb11b9af69b85719093be154bd9a9a23792d1ecb64f70b34dd69fdbec6c7cdf7048d62c6a6d94ee9f65e78aafad2ea45d94765e285a18485b879f814fde17c6b01b"
}
```
Success:
```
{
    "type": "upsert-code",
    "data": {
        "code": "ABCD"
    }
}
```
Error:
```
{"error":"code upsert failed:Not code owner"}
```
# Notes


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
