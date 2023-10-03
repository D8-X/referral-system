# referral-system
Referral system

# Todos

- walk back in time block-time
- edit referral?
- fool-proof loops/cycles in chain
- get queries

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

## Refer
/refer
passOnPercTDF is two-digit format, for example, 2.5% is sent as 250, 65% as 6500

```
{
    "parentAddr": "0x5A09217F6D36E73eE5495b430e889f8c57876Ef3",
    "referToAddr": "0x9d5aaB428e98678d0E645ea4AeBd25f744341a05",
    "passOnPercTDF": 225,
    "createdOn": 1696166434,
    "signature": "0x09bbe815eba739e28c665c6637e7e45dca03eed00d0eaffb1890713c8b3f9e760d41102d5d6885724bd53c7fc0bedcce8dfebe020464c234c1c1d4d194090f071c"
}
```
Success:
```
{
    "type": "referral-code",
    "data": {
        "referToAddr": "0x863ad9ce46acf07fd9390147b619893461036194"
    }
}
```
Error:
only one occurrence as child allowed:
`{"error":"referral failed:Refer to addr already in use"}`
`{"error":"referral failed:Not an agency"}`


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

## done: select-referral-code

## done: refer
as agency or broker (child in referral-chain or broker) we can add a new child.
the child (refer to address) is not allowed to be already in the quueue
This needs to be signed by the agency/broker and we need to check the signature
-Edit?

- done: upsert-referral-code
Anyone can create a code. The parent is either a child in the referral_chain, or
the broker

## Contracts
Generate the ABI:
`abigen --abi src/contracts/abi/MultiPay.json --pkg contracts --type MultiPay --out multi_pay.go`
